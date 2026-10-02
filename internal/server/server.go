// Package server answers HTTP requests: it resolves hosts to the apex or a
// site, renders the HTML shells and serves the APIs.
package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

// Server is the HTTP handler of SitRep.
type Server struct {
	log      xlog.Logger
	cfg      config.Config
	db       *store.DB
	assets   *assets
	authAPI  http.Handler
	adminAPI http.Handler
}

// New returns a Server. It fails if the frontend build is missing.
func New(
	log xlog.Logger,
	cfg config.Config,
	db *store.DB,
	core *auth.Core,
) (*Server, error) {
	a, err := newAssets()
	if err != nil {
		return nil, err
	}
	return newServer(log, cfg, db, core, a), nil
}

func newServer(
	log xlog.Logger,
	cfg config.Config,
	db *store.DB,
	core *auth.Core,
	a *assets,
) *Server {
	s := &Server{
		log:     log,
		cfg:     cfg,
		db:      db,
		assets:  a,
		authAPI: core.Handler(),
	}

	api := http.NewServeMux()
	api.HandleFunc("GET /api/admin/settings", s.getSettings)
	api.HandleFunc("PUT /api/admin/settings", s.putSettings)
	s.adminAPI = core.Guard(core.CSRF(api))

	return s
}

// Handler returns the server's handler, with access logging.
func (s *Server) Handler() http.Handler {
	return httpx.AccessLog(s.log, s.cfg.TrustProxy, s)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	path := r.URL.Path
	switch {
	case path == "/healthz":
		s.health(w)
		return
	case strings.HasPrefix(path, "/assets/"):
		s.assets.ServeHTTP(w, r)
		return
	}

	info := httpx.Effective(r, s.cfg.TrustProxy)
	if slices.Contains(s.cfg.BaseDomains, info.Host) {
		s.serveApex(w, r, info)
		return
	}

	site, err := s.siteByHost(info.Host)
	switch {
	case err != nil:
		httpx.WriteError(w, r, s.log, err)
	case site == nil:
		http.NotFound(w, r)
	default:
		s.serveSite(w, r, info, site, "")
	}
}

func (s *Server) health(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := s.db.Check(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("database unavailable\n"))
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}

// siteByHost returns the subdomain-mode or custom-domain site served on
// host, or nil.
func (s *Server) siteByHost(host string) (*model.Site, error) {
	for _, base := range s.cfg.BaseDomains {
		label, ok := strings.CutSuffix(host, "."+base)
		if ok && !strings.Contains(label, ".") {
			site, err := s.db.SiteByRoute(model.Route{
				Mode: model.RouteSubdomain,
				Slug: label,
			})
			if site != nil || err != nil {
				return site, err
			}
		}
	}
	return s.db.SiteByRoute(model.Route{
		Mode:   model.RouteCustom,
		Domain: host,
	})
}

// serveApex serves a base domain host: the admin console, auth and admin
// API, path-mode sites and the landing pages.
func (s *Server) serveApex(
	w http.ResponseWriter,
	r *http.Request,
	info httpx.Info,
) {
	path := r.URL.Path
	switch {
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		s.serveAdmin(w, r)
		return
	case strings.HasPrefix(path, "/auth/"):
		s.authAPI.ServeHTTP(w, r)
		return
	case strings.HasPrefix(path, "/api/admin/"):
		s.adminAPI.ServeHTTP(w, r)
		return
	}

	if slug, _, _ := strings.Cut(path[1:], "/"); slug != "" {
		site, err := s.db.SiteByRoute(model.Route{
			Mode: model.RoutePath,
			Slug: slug,
		})
		if err != nil {
			httpx.WriteError(w, r, s.log, err)
			return
		}

		if site != nil {
			s.serveSite(w, r, info, site, "/"+slug)
			return
		}
	}

	s.serveLanding(w, r, info)
}

// negotiator returns a function that negotiates the visitor's language
// among langs.
func negotiator(r *http.Request, langs model.Languages) func() string {
	return func() string {
		var cookie string
		if c, err := r.Cookie("lang"); err == nil {
			cookie = c.Value
		}
		accept := r.Header.Get("Accept-Language")
		return i18n.Negotiate(langs.Enabled, langs.Primary, cookie, accept)
	}
}

// redirect sends a 302 to target below base, keeping the query.
func redirect(w http.ResponseWriter, r *http.Request, base string, res resolved) {
	target := base + res.redirect
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	if res.negotiated {
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("Vary", "Accept-Language, Cookie")
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) serveSite(
	w http.ResponseWriter,
	r *http.Request,
	info httpx.Info,
	site *model.Site,
	base string,
) {
	settings, err := s.db.Settings()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := site.Languages.Effective()
	path := strings.TrimPrefix(r.URL.Path, base)
	res := routePage(path, langs, false, negotiator(r, langs))
	if res.redirect != "" {
		redirect(w, r, base, res)
		return
	}

	name := site.Name.Resolve(res.lang, langs)
	c := i18n.Get(res.lang)
	status := http.StatusOK
	title := ""
	switch res.page.kind {
	case pageOverview:
	case pageArchive:
		title = c.T("incidents.all", nil)
	default:
		res.page.kind = pageNotFound
		status = http.StatusNotFound
		title = c.T("page.notFound", nil)
	}

	sh := pageShell(info, base, langs, res, title, name)
	sh.Theme = theme(r, settings.DefaultTheme)
	sh.Assets = s.assets.entry("public")
	sh.Bootstrap.Mode = "site"
	sh.Bootstrap.SiteID = site.ID
	writeShell(w, status, sh)
}

func (s *Server) serveLanding(
	w http.ResponseWriter,
	r *http.Request,
	info httpx.Info,
) {
	settings, err := s.db.Settings()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := settings.Languages.Effective()
	res := routePage(r.URL.Path, langs, true, negotiator(r, langs))
	if res.redirect != "" {
		redirect(w, r, "", res)
		return
	}

	c := i18n.Get(res.lang)
	status := http.StatusOK
	title := ""
	if res.page.kind != pageOverview {
		res.page.kind = pageNotFound
		status = http.StatusNotFound
		title = c.T("page.notFound", nil)
	}

	sh := pageShell(info, "", langs, res, title, c.T("page.landing", nil))
	sh.Theme = theme(r, settings.DefaultTheme)
	sh.Assets = s.assets.entry("public")
	sh.Bootstrap.Mode = "landing"
	writeShell(w, status, sh)
}

// serveAdmin serves the admin console's shell for every path below /admin.
// Its language is any supported one: the visitor's choice, else the
// browser's, else the instance's primary language.
func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.Settings()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := model.Languages{
		Enabled: i18n.Supported(),
		Primary: settings.Languages.Effective().Primary,
	}
	lang := negotiator(r, langs)()
	writeShell(w, http.StatusOK, shell{
		Theme:  theme(r, settings.DefaultTheme),
		Title:  i18n.Get(lang).T("page.admin", nil),
		Assets: s.assets.entry("admin"),
		Bootstrap: bootstrap{
			Mode:      "admin",
			BasePath:  "/admin",
			Lang:      lang,
			Languages: langs.Enabled,
			Primary:   langs.Primary,
		},
	})
}

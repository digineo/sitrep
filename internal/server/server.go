// Package server answers HTTP requests: it resolves hosts to the apex or a
// site, renders the HTML shells and serves the APIs.
package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/poller"
	"github.com/digineo/sitrep/internal/store"
)

// Server is the HTTP handler of SitRep.
type Server struct {
	log       xlog.Logger
	cfg       config.Config
	db        *store.DB
	poller    *poller.Poller
	assets    *assets
	authAPI   http.Handler
	adminAPI  http.Handler
	publicAPI http.Handler
}

// New returns a Server. It fails if the frontend build is missing.
func New(
	log xlog.Logger,
	cfg config.Config,
	db *store.DB,
	core *auth.Core,
	p *poller.Poller,
) (*Server, error) {
	a, err := newAssets()
	if err != nil {
		return nil, err
	}
	return newServer(log, cfg, db, core, p, a), nil
}

func newServer(
	log xlog.Logger,
	cfg config.Config,
	db *store.DB,
	core *auth.Core,
	p *poller.Poller,
	a *assets,
) *Server {
	s := &Server{
		log:     log,
		cfg:     cfg,
		db:      db,
		poller:  p,
		assets:  a,
		authAPI: core.Handler(),
	}

	admin := http.NewServeMux()
	handle := func(pattern string, h http.HandlerFunc) {
		admin.Handle(pattern, core.CSRF(h, "application/json"))
	}

	handle("GET /api/admin/settings", s.getSettings)
	handle("PUT /api/admin/settings", s.putSettings)
	handle("GET /api/admin/datasource-types", s.listTypes)
	handle("GET /api/admin/datasources", s.listDataSources)
	handle("POST /api/admin/datasources", s.createDataSource)
	handle("POST /api/admin/datasources/test", s.testUnsaved)
	handle("GET /api/admin/datasources/{id}", s.getDataSource)
	handle("PUT /api/admin/datasources/{id}", s.putDataSource)
	handle("DELETE /api/admin/datasources/{id}", s.deleteDataSource)
	handle("POST /api/admin/datasources/{id}/test", s.testDataSource)
	// Routes of data source types, e.g. the Prometheus discovery proxy,
	// which forwards form-encoded requests.
	dsRoute := core.CSRF(
		http.HandlerFunc(s.dataSourceRoute),
		"application/x-www-form-urlencoded",
	)
	admin.Handle("/api/admin/datasources/{id}/{type}/", dsRoute)
	handle("GET /api/admin/sites", s.listSites)
	handle("POST /api/admin/sites", s.createSite)
	handle("GET /api/admin/sites/{site}", s.getSite)
	handle("PUT /api/admin/sites/{site}", s.putSite)
	handle("DELETE /api/admin/sites/{site}", s.deleteSite)
	handle("GET /api/admin/sites/{site}/preview", s.previewSite)
	handle("GET /api/admin/sites/{site}/panels", s.listPanels)
	handle("POST /api/admin/sites/{site}/panels", s.createPanel)
	handle("PUT /api/admin/sites/{site}/panel-order", s.reorderPanels)
	handle("POST /api/admin/sites/{site}/panel-preview", s.previewPanel)
	handle("GET /api/admin/sites/{site}/panels/{panel}", s.getPanel)
	handle("PUT /api/admin/sites/{site}/panels/{panel}", s.putPanel)
	handle("DELETE /api/admin/sites/{site}/panels/{panel}", s.deletePanel)
	handle("GET /api/admin/sites/{site}/incidents", s.listIncidents)
	handle("POST /api/admin/sites/{site}/incidents", s.createIncident)
	handle("GET /api/admin/sites/{site}/incidents/{incident}", s.getIncident)
	handle("PUT /api/admin/sites/{site}/incidents/{incident}", s.putIncident)
	handle("DELETE /api/admin/sites/{site}/incidents/{incident}", s.deleteIncident)
	handle("POST /api/admin/sites/{site}/incidents/{incident}/updates", s.addUpdate)
	handle(
		"PUT /api/admin/sites/{site}/incidents/{incident}/updates/{update}",
		s.putUpdate,
	)
	handle(
		"DELETE /api/admin/sites/{site}/incidents/{incident}/updates/{update}",
		s.deleteUpdate,
	)
	handle("POST /api/admin/markdown", s.previewMarkdown)
	handle("POST /api/admin/svg", s.sanitizeSVG)
	s.adminAPI = core.Guard(admin)

	public := http.NewServeMux()
	public.HandleFunc("GET /api/public/sites/{site}", s.publicSite)
	public.HandleFunc("GET /api/public/sites/{site}/incidents", s.publicArchive)
	public.HandleFunc(
		"GET /api/public/sites/{site}/incidents/{incident}",
		s.publicIncident,
	)
	public.HandleFunc("GET /api/public/sites/{site}/legal/{kind}", s.publicLegal)
	public.HandleFunc("GET /api/public/sites/{site}/logo/{file}", s.publicLogo)
	public.HandleFunc("GET /api/public/legal", s.publicLegalLinks)
	public.HandleFunc("GET /api/public/legal/{kind}", s.publicLegal)
	public.HandleFunc("GET /api/public/landing", s.publicLanding)
	s.publicAPI = public

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
	case strings.HasPrefix(path, "/api/public/"):
		s.publicAPI.ServeHTTP(w, r)
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

	online := site.Online()
	switch res.page.kind {
	case pageFeed, pageJSON:
		switch {
		case !online:
			err := apierr.New(http.StatusServiceUnavailable, apierr.SiteUnavailable)
			httpx.WriteError(w, r, s.log, err)
		case res.page.kind == pageFeed:
			s.serveFeed(w, r, info, site, base, res.lang)
		default:
			s.serveIncidentsJSON(w, r, site, res.lang)
		}
		return
	}

	name := site.Name.Resolve(res.lang, langs)
	c := i18n.Get(res.lang)
	status := http.StatusOK
	title := ""
	legal := res.page.kind == pageImprint || res.page.kind == pagePrivacy
	if !online && !legal {
		// Unavailable sites show only their legal pages.
		status = http.StatusServiceUnavailable
	} else {
		switch res.page.kind {
		case pageArchive:
			title = c.T("incidents.all", nil)
			incidents, err := s.db.Incidents(site.ID)
			if err != nil {
				httpx.WriteError(w, r, s.log, err)
				return
			}

			pageNum := r.URL.Query().Get("page")
			_, _, _, ok := paginate(pageNum, len(incidents), archivePageSize)
			if !ok {
				res.page.kind = pageNotFound
				status = http.StatusNotFound
				title = c.T("page.notFound", nil)
			}
		case pageIncident:
			inc, err := s.db.Incident(site.ID, res.page.id)
			e, ok := errors.AsType[*apierr.Error](err)
			if ok && e.Status == http.StatusNotFound {
				res.page.kind = pageNotFound
				status = http.StatusNotFound
				title = c.T("incidents.notFound", nil)
				break
			} else if err != nil {
				httpx.WriteError(w, r, s.log, err)
				return
			}

			title = inc.Title.Resolve(res.lang, langs)
		case pageImprint, pagePrivacy:
			var handled bool
			if title, handled = legalShell(w, r, res, site, settings); handled {
				return
			}
		}
		if title == "" && res.page.kind != pageOverview {
			res.page.kind = pageNotFound
			status = http.StatusNotFound
			title = c.T("page.notFound", nil)
		}
	}

	sh := pageShell(info, base, langs, res, title, name)
	if online {
		feed := pagePath(page{kind: pageFeed}, res.lang, len(langs.Enabled) > 1)
		sh.Feed = info.Origin + base + feed
	}

	def := settings.DefaultTheme
	if site.Theme != "" && site.Theme != "inherit" {
		def = site.Theme
	}

	sh.Theme = theme(r, def)
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
	if res.page.kind == pageImprint || res.page.kind == pagePrivacy {
		var handled bool
		if title, handled = legalShell(w, r, res, nil, settings); handled {
			return
		}
	}
	if title == "" && res.page.kind != pageOverview {
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
			Mode:           "admin",
			BasePath:       "/admin",
			Lang:           lang,
			Languages:      langs.Enabled,
			Primary:        langs.Primary,
			BaseDomains:    s.cfg.BaseDomains,
			DefaultRefresh: model.FormatDuration(s.cfg.DefaultRefresh),
		},
	})
}

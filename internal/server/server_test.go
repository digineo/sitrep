package server

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/poller"
	"github.com/digineo/sitrep/internal/store"
)

// testProvider signs in everyone who posts to /auth/test/login: the
// subject and email of the body, or Ann.
type testProvider struct{}

func (testProvider) Method() auth.Method { return auth.MethodCredentials }
func (testProvider) Available() bool     { return true }
func (testProvider) Routes(mux *http.ServeMux, core *auth.Core) {
	login := func(w http.ResponseWriter, r *http.Request) {
		id := auth.Identity{
			Subject:     "ann",
			DisplayName: "Ann",
		}
		if err := json.NewDecoder(r.Body).Decode(&id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := core.Login(w, r, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	mux.HandleFunc("POST /auth/test/login", login)
}

type fixture struct {
	t       *testing.T
	cfg     config.Config
	db      *store.DB
	dbPath  string
	poller  *poller.Poller
	srv     http.Handler
	session *http.Cookie
}

var testKey = bytes.Repeat([]byte{42}, 32)

// newFixture serves a database with three sites. Panels without their own
// refresh are polled every 50ms.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sitrep.db")
	db, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := config.Config{
		BaseDomains:    []string{"status.example.com", "sitrep.localhost"},
		SecretKey:      testKey,
		DefaultRefresh: 30 * time.Second,
	}
	core := auth.NewCore(
		xlog.NewDiscard(),
		db,
		"test",
		testProvider{},
		time.Hour,
		false,
	)
	p := poller.New(xlog.NewDiscard(), db, testKey, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	f := &fixture{
		t:      t,
		cfg:    cfg,
		db:     db,
		dbPath: path,
		poller: p,
		srv:    newServer(xlog.NewDiscard(), cfg, db, core, p, testAssets()),
	}
	for _, site := range []*model.Site{
		{
			Name: model.Text{"de": "Acme </script><script>alert(1)</script>"},
			Languages: model.Languages{
				Enabled: []string{"de"},
				Primary: "de",
			},
			Route: model.Route{
				Mode: model.RoutePath,
				Slug: "acme",
			},
		},
		{
			Name: model.Text{
				"en": "Beta",
				"de": "Beta DE",
			},
			Languages: model.Languages{
				Enabled: []string{"de", "en"},
				Primary: "en",
			},
			Route: model.Route{
				Mode: model.RouteSubdomain,
				Slug: "beta",
			},
		},
		{
			Name: model.Text{"en": "Gamma"},
			Languages: model.Languages{
				Enabled: []string{"en"},
				Primary: "en",
			},
			Route: model.Route{
				Mode:   model.RouteCustom,
				Domain: "status.gamma.org",
			},
		},
	} {
		require.NoError(t, db.CreateSite(site))
	}
	return f
}

func (f *fixture) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, r)
	return w
}

func (f *fixture) get(
	host, path string,
	headers ...string,
) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = host
	for i := 0; i < len(headers); i += 2 {
		r.Header.Add(headers[i], headers[i+1])
	}
	return f.do(r)
}

var bootstrapPattern = regexp.MustCompile(
	`<script type="application/json" id="bootstrap">(.*)</script>`,
)

func parseBootstrap(t *testing.T, body string) bootstrap {
	t.Helper()
	m := bootstrapPattern.FindStringSubmatch(body)
	require.NotNil(t, m, body)
	var b bootstrap
	require.NoError(t, json.Unmarshal([]byte(m[1]), &b))
	return b
}

func TestHostRouting(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	tests := []struct {
		host, path string
		status     int
		location   string
		mode       string
	}{
		{"status.example.com", "/", http.StatusFound, "/en/", ""},
		{"status.example.com", "/en/", http.StatusOK, "", "landing"},
		{"status.example.com", "/nope", http.StatusNotFound, "", "landing"},
		{"Status.Example.com.:2607", "/en/", http.StatusOK, "", "landing"},
		{"status.example.com", "/admin", http.StatusOK, "", "admin"},
		{"status.example.com", "/admin/sites/x", http.StatusOK, "", "admin"},
		{"status.example.com", "/administrator", http.StatusNotFound, "", "landing"},
		{"status.example.com", "/acme/", http.StatusOK, "", "site"},
		{"sitrep.localhost", "/acme/", http.StatusOK, "", "site"},
		{"status.example.com", "/acme", http.StatusFound, "/acme/", ""},
		{"status.example.com", "/acme/de/", http.StatusFound, "/acme/", ""},
		{"status.example.com", "/acme/incidents", http.StatusOK, "", "site"},
		{"status.example.com", "/acme/nope", http.StatusNotFound, "", "site"},
		{"beta.status.example.com", "/", http.StatusFound, "/en/", ""},
		{"beta.sitrep.localhost", "/de/", http.StatusOK, "", "site"},
		{"beta.status.example.com", "/admin", http.StatusNotFound, "", "site"},
		{"beta.status.example.com", "/acme/", http.StatusNotFound, "", "site"},
		{"status.gamma.org", "/", http.StatusOK, "", "site"},
		{"STATUS.gamma.org.", "/incidents", http.StatusOK, "", "site"},
		{"acme.status.example.com", "/", http.StatusNotFound, "", ""},
		{"x.beta.status.example.com", "/", http.StatusNotFound, "", ""},
		{"example.com", "/", http.StatusNotFound, "", ""},
		{"gamma.org", "/", http.StatusNotFound, "", ""},
	}
	for _, tt := range tests {
		w := f.get(tt.host, tt.path)
		name := tt.host + tt.path
		assert.Equal(tt.status, w.Code, name)
		assert.Equal(tt.location, w.Header().Get("Location"), name)
		assert.Equal("nosniff", w.Header().Get("X-Content-Type-Options"), name)
		if tt.mode != "" {
			assert.Equal(tt.mode, parseBootstrap(t, w.Body.String()).Mode, name)
		}
	}
}

func TestApexOnlyRoutes(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/auth/session", "/api/admin/settings"} {
		code := f.get("status.example.com", path).Code
		assert.NotEqual(t, http.StatusNotFound, code, path)
		for _, host := range []string{
			"beta.status.example.com",
			"status.gamma.org",
		} {
			w := f.get(host, path)
			assert.Equal(t, http.StatusNotFound, w.Code, host+path)
			assert.NotContains(t, w.Body.String(), "unauthorized", host+path)
		}
	}
}

func TestEveryHostRoutes(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	for _, host := range []string{
		"status.example.com",
		"beta.status.example.com",
		"status.gamma.org",
		"unknown.example",
	} {
		w := f.get(host, "/healthz")
		assert.Equal(http.StatusOK, w.Code, host)
		assert.Equal("ok\n", w.Body.String(), host)
		assert.Equal(http.StatusOK, f.get(host, "/assets/admin-abc.js").Code, host)
	}

	require.NoError(f.db.Close())
	code := f.get("status.example.com", "/healthz").Code
	assert.Equal(http.StatusServiceUnavailable, code)
}

func TestTLSAuthorize(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	gamma, err := f.db.SiteByRoute(model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.gamma.org",
	})
	require.NoError(err)
	gamma.Availability = model.AvailabilityPaused
	require.NoError(f.db.UpdateSite(gamma))

	tests := map[string]int{
		"status.example.com":           http.StatusOK,
		"SitRep.Localhost.":            http.StatusOK,
		"beta.status.example.com":      http.StatusOK,
		"beta.sitrep.localhost:443":    http.StatusOK,
		"status.gamma.org":             http.StatusOK,
		"":                             http.StatusNotFound,
		"example.com":                  http.StatusNotFound,
		"acme.status.example.com":      http.StatusNotFound,
		"x.beta.status.example.com":    http.StatusNotFound,
		"beta.status.example.com:none": http.StatusNotFound,
		"status_gamma.org":             http.StatusNotFound,
		"[::1]:443":                    http.StatusNotFound,
	}
	for domain, status := range tests {
		for _, host := range []string{
			"status.example.com",
			"status.gamma.org",
			"unknown.example",
		} {
			w := f.get(host, "/tls/authorize?domain="+url.QueryEscape(domain))
			assert.Equal(status, w.Code, domain+" on "+host)
			if status == http.StatusOK {
				assert.Empty(w.Body.String(), domain)
			}
		}
	}
}

func TestNegotiatedRedirects(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	w := f.get(
		"beta.status.example.com",
		"/incidents?page=2",
		"Accept-Language", "de-AT, en;q=0.5",
	)
	assert.Equal(http.StatusFound, w.Code)
	assert.Equal("/de/incidents?page=2", w.Header().Get("Location"))
	assert.Equal("private, no-cache", w.Header().Get("Cache-Control"))
	assert.Equal("Accept-Language, Cookie", w.Header().Get("Vary"))

	w = f.get(
		"beta.status.example.com",
		"/",
		"Accept-Language", "de",
		"Cookie", "lang=en",
	)
	assert.Equal("/en/", w.Header().Get("Location"), "the cookie wins")

	w = f.get("beta.status.example.com", "/de")
	assert.Equal("/de/", w.Header().Get("Location"))
	assert.Empty(w.Header().Get("Vary"), "not negotiated")
	assert.Empty(w.Header().Get("Cache-Control"))
}

func TestSiteShell(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.get(
		"beta.status.example.com:8080",
		"/de/incidents",
		"Cookie", "theme=dark",
	)
	require.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(body, `<html lang="de" data-theme="dark">`)
	assert.Contains(body, "<title>Alle Vorfälle · Beta DE</title>")
	assert.Contains(body, `<link rel="canonical" href="http://beta.status.example.com:8080/de/incidents">`)
	assert.Contains(body, `<link rel="alternate" hreflang="de" href="http://beta.status.example.com:8080/de/incidents">`)
	assert.Contains(body, `<link rel="alternate" hreflang="en" href="http://beta.status.example.com:8080/en/incidents">`)
	assert.Contains(body, `<link rel="alternate" hreflang="x-default" href="http://beta.status.example.com:8080/incidents">`)
	assert.Contains(body, `<script type="module" src="/assets/public-abc.js"></script>`)
	assert.Contains(body, "<link rel=\"stylesheet\" href=\"/assets/shared-abc.css\">\n<link rel=\"stylesheet\" href=\"/assets/public-abc.css\">")
	assert.Contains(body, `<link rel="icon" type="image/svg+xml" href="data:image/svg`)

	site, err := f.db.SiteByRoute(model.Route{
		Mode: model.RouteSubdomain,
		Slug: "beta",
	})
	require.NoError(err)
	want := bootstrap{
		Mode:      "site",
		SiteID:    site.ID,
		BasePath:  "",
		Lang:      "de",
		Languages: []string{"de", "en"},
		Primary:   "en",
	}
	assert.Equal(want, parseBootstrap(t, body))

	h := w.Header()
	assert.Equal("text/html; charset=utf-8", h.Get("Content-Type"))
	assert.Equal(cspPublic, h.Get("Content-Security-Policy"))
	assert.NotContains(cspPublic, "unsafe-eval")
	assert.NotContains(cspPublic, "script-src")
	assert.Equal("strict-origin-when-cross-origin", h.Get("Referrer-Policy"))
}

func TestShellEscapesBootstrap(t *testing.T) {
	w := httptest.NewRecorder()
	writeShell(w, http.StatusOK, shell{Bootstrap: bootstrap{BasePath: "</script><script>alert(1)</script><!--"}})
	body := w.Body.String()
	assert.NotContains(t, body, "<script>alert")
	assert.NotContains(t, body, "<!--")
	b := parseBootstrap(t, body)
	assert.Equal(t, "</script><script>alert(1)</script><!--", b.BasePath)
}

func TestShellEscapesTitle(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.get("status.example.com", "/acme/")
	require.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(body, "<script>alert")
	escaped := html.EscapeString("Acme </script><script>alert(1)</script>")
	assert.Contains(body, "<title>"+escaped+"</title>")
	b := parseBootstrap(t, body)
	assert.Equal("/acme", b.BasePath)
	alt := regexp.MustCompile(`<link rel="alternate" hreflang`).FindString(body)
	assert.Empty(alt, "one language has no alternates")
	assert.Contains(body, `<link rel="alternate" type="application/atom+xml" href="http://status.example.com/acme/feed.atom">`)
	assert.Contains(body, `<link rel="canonical" href="http://status.example.com/acme/">`)
}

func TestSiteShellTheme(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site, err := f.db.SiteByRoute(model.Route{
		Mode: model.RoutePath,
		Slug: "acme",
	})
	require.NoError(err)
	assert.Contains(
		f.get("status.example.com", "/acme/").Body.String(),
		`<html lang="de">`,
		"inherits the instance's system default",
	)

	site.Theme = "dark"
	require.NoError(f.db.UpdateSite(site))
	assert.Contains(
		f.get("status.example.com", "/acme/").Body.String(),
		`<html lang="de" data-theme="dark">`,
	)
	light := f.get("status.example.com", "/acme/", "Cookie", "theme=light")
	assert.Contains(
		light.Body.String(),
		`data-theme="light"`,
		"the visitor's choice comes first",
	)

	site.Theme = "system"
	require.NoError(f.db.UpdateSite(site))
	require.NoError(f.db.PutSettings(model.Settings{
		Languages:    model.DefaultSettings().Languages,
		DefaultTheme: "dark",
	}))
	assert.Contains(
		f.get("status.example.com", "/acme/").Body.String(),
		`<html lang="de">`,
		"the site's choice before the instance's",
	)
}

func TestNotFoundShell(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.get("status.gamma.org", "/nope")
	require.Equal(http.StatusNotFound, w.Code)
	body := w.Body.String()
	assert.Contains(body, "<title>Page not found · Gamma</title>")
	assert.NotContains(body, `rel="canonical"`)
	assert.Equal("site", parseBootstrap(t, body).Mode)

	for _, path := range []string{"/incidents/0192", "/imprint"} {
		code := f.get("status.gamma.org", path).Code
		assert.Equal(http.StatusNotFound, code, path)
	}
}

func TestLandingShell(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	require.NoError(f.db.PutSettings(model.Settings{
		Languages: model.Languages{
			Enabled: []string{"de"},
			Primary: "de",
		},
		DefaultTheme: "light",
	}))

	w := f.get("status.example.com", "/")
	require.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(body, `<html lang="de" data-theme="light">`)
	assert.Contains(body, "<title>Statusseiten</title>")
	want := bootstrap{
		Mode:      "landing",
		Lang:      "de",
		Languages: []string{"de"},
		Primary:   "de",
	}
	assert.Equal(want, parseBootstrap(t, body))

	w = f.get("status.example.com", "/", "Cookie", "theme=system")
	assert.Contains(
		w.Body.String(),
		`<html lang="de">`,
		"the visitor's choice wins over the default",
	)

	w = f.get("status.example.com", "/", "Cookie", "theme=pink")
	assert.Contains(w.Body.String(), `<html lang="de" data-theme="light">`)
}

func TestAdminShell(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.get("status.example.com", "/admin/settings", "Accept-Language", "de-DE")
	require.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(body, `<html lang="de">`)
	assert.Contains(body, "<title>SitRep-Verwaltung</title>")
	assert.Equal(cspAdmin, w.Header().Get("Content-Security-Policy"))
	assert.Contains(cspAdmin, "frame-ancestors 'none'")
	want := bootstrap{
		Mode:           "admin",
		BasePath:       "/admin",
		Lang:           "de",
		Languages:      []string{"de", "en"},
		Primary:        "en",
		BaseDomains:    []string{"status.example.com", "sitrep.localhost"},
		DefaultRefresh: "30s",
	}
	assert.Equal(want, parseBootstrap(t, body))

	w = f.get(
		"status.example.com",
		"/admin",
		"Accept-Language", "de-DE",
		"Cookie", "lang=en",
	)
	assert.Equal("en", parseBootstrap(t, w.Body.String()).Lang)
}

// TestSecurityHeaders checks every kind of response: all are nosniff, and
// every HTML shell has the content security and referrer policies. Only
// the console's shell forbids framing.
func TestSecurityHeaders(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	assert.Equal("default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; form-action 'self'", cspPublic)
	assert.Equal(cspPublic+"; frame-ancestors 'none'", cspAdmin)

	f := newFixture(t)
	gamma, err := f.db.SiteByRoute(model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.gamma.org",
	})
	require.NoError(err)
	gamma.Availability = model.AvailabilityOffline
	require.NoError(f.db.UpdateSite(gamma))

	tests := []struct {
		host, path string
		status     int
		csp        string // "" for responses that are not shells
	}{
		{"status.example.com", "/en/", http.StatusOK, cspPublic},
		{"status.example.com", "/nope", http.StatusNotFound, cspPublic},
		{"status.example.com", "/acme/", http.StatusOK, cspPublic},
		{"beta.status.example.com", "/en/nope", http.StatusNotFound, cspPublic},
		{"status.gamma.org", "/", http.StatusServiceUnavailable, cspPublic},
		{"status.example.com", "/admin/", http.StatusOK, cspAdmin},
		{"status.example.com", "/acme", http.StatusFound, ""},
		{"status.example.com", "/healthz", http.StatusOK, ""},
		{"unknown.example", "/tls/authorize?domain=example.com", http.StatusNotFound, ""},
		{"status.example.com", "/assets/admin-abc.js", http.StatusOK, ""},
		{"status.example.com", "/assets/missing.js", http.StatusNotFound, ""},
		{"status.example.com", "/api/public/sites/x", http.StatusBadRequest, ""},
		{"status.example.com", "/api/admin/settings", http.StatusUnauthorized, ""},
		{"status.example.com", "/auth/session", http.StatusOK, ""},
		{"beta.status.example.com", "/en/feed.atom", http.StatusOK, ""},
		{"status.gamma.org", "/incidents.json", http.StatusServiceUnavailable, ""},
		{"unknown.example", "/", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		w := f.get(tt.host, tt.path)
		name := tt.host + tt.path
		h := w.Header()
		assert.Equal(tt.status, w.Code, name)
		assert.Equal("nosniff", h.Get("X-Content-Type-Options"), name)
		if tt.csp != "" {
			assert.Equal("text/html; charset=utf-8", h.Get("Content-Type"), name)
			assert.Equal(tt.csp, h.Get("Content-Security-Policy"), name)
			referrer := h.Get("Referrer-Policy")
			assert.Equal("strict-origin-when-cross-origin", referrer, name)
		}
	}
}

func TestSettingsAPI(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	request := func(
		method, body string,
		cookie *http.Cookie,
	) *httptest.ResponseRecorder {
		r := httptest.NewRequest(
			method,
			"http://status.example.com/api/admin/settings",
			strings.NewReader(body),
		)
		r.Header.Set("Origin", "http://status.example.com")
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		return f.do(r)
	}

	assert.Equal(http.StatusUnauthorized, request(http.MethodGet, "", nil).Code)

	w := f.do(func() *http.Request {
		r := httptest.NewRequest(
			http.MethodPost,
			"http://status.example.com/auth/test/login",
			strings.NewReader("{}"),
		)
		r.Header.Set("Origin", "http://status.example.com")
		r.Header.Set("Content-Type", "application/json")
		return r
	}())

	cookies := w.Result().Cookies()
	require.Len(cookies, 1)
	session := cookies[0]

	w = request(http.MethodGet, "", session)
	require.Equal(http.StatusOK, w.Code)
	assert.JSONEq(`{"languages": {"enabled": ["en", "de"], "primary": "en"}, "defaultTheme": "system",
		"legal": {"imprint": {"mode": "none"}, "privacy": {"mode": "none"}}}`, w.Body.String())

	w = request(http.MethodPut, `{"languages": {"enabled": ["de"], "primary": "en"}, "defaultTheme": "dark"}`, session)
	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Contains(w.Body.String(), `{"path":"languages.primary","code":"primary_not_enabled"}`)

	w = request(http.MethodPut, `{"languages": {"enabled": ["de"], "primary": "de"}, "defaultTheme": "dark", "extra": 1}`, session)
	assert.Equal(http.StatusBadRequest, w.Code, "unknown fields are rejected")

	w = request(http.MethodPut, `{"languages": {"enabled": ["de", "en"], "primary": "de"}, "defaultTheme": "dark"}`, session)
	require.Equal(http.StatusOK, w.Code)
	s, err := f.db.Settings()
	require.NoError(err)
	want := model.Settings{
		Languages: model.Languages{
			Enabled: []string{"de", "en"},
			Primary: "de",
		},
		DefaultTheme: "dark",
		Legal: model.Legal{
			Imprint: model.LegalPage{Mode: "none"},
			Privacy: model.LegalPage{Mode: "none"},
		},
	}
	assert.Equal(want, s, "legal pages default to none")

	r := httptest.NewRequest(
		http.MethodPut,
		"http://status.example.com/api/admin/settings",
		strings.NewReader(`{}`),
	)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(session)
	assert.Equal(http.StatusForbidden, f.do(r).Code, "CSRF protection")
}

func TestPromotedSite(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.admin(http.MethodPut, "/api/admin/settings", obj{
		"languages": obj{
			"enabled": []string{"en"},
			"primary": "en",
		},
		"defaultTheme": "system",
		"landingSite":  "0192",
	})
	want := map[string]string{"landingSite": "not_found"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	promote := func(route model.Route) *model.Site {
		t.Helper()
		site, err := f.db.SiteByRoute(route)
		require.NoError(err)
		settings, err := f.db.Settings()
		require.NoError(err)
		settings.LandingSite = site.ID
		require.NoError(f.db.PutSettings(settings))
		return site
	}

	beta := promote(model.Route{
		Mode: model.RouteSubdomain,
		Slug: "beta",
	})
	tests := []struct {
		host, path string
		status     int
		location   string
		mode       string
	}{
		{"status.example.com", "/", http.StatusFound, "/en/", ""},
		{"sitrep.localhost", "/en/incidents", http.StatusOK, "", "site"},
		{"status.example.com", "/nope", http.StatusNotFound, "", "site"},
		{"status.example.com", "/acme/", http.StatusOK, "", "site"},
		{"status.example.com", "/admin", http.StatusOK, "", "admin"},
		{"beta.status.example.com", "/", http.StatusFound, "http://status.example.com/", ""},
		{"beta.sitrep.localhost:2607", "/de/incidents?page=2", http.StatusFound, "http://sitrep.localhost:2607/de/incidents?page=2", ""},
		{"beta.status.example.com", "/en/feed.atom", http.StatusFound, "http://status.example.com/en/feed.atom", ""},
		{"beta.status.example.com", "//evil.example/", http.StatusFound, "http://status.example.com//evil.example/", ""},
		{"unknown.example", "/tls/authorize?domain=beta.status.example.com", http.StatusOK, "", ""},
	}
	for _, tt := range tests {
		w := f.get(tt.host, tt.path)
		name := tt.host + tt.path
		assert.Equal(tt.status, w.Code, name)
		assert.Equal(tt.location, w.Header().Get("Location"), name)
		if tt.mode != "" {
			assert.Equal(tt.mode, parseBootstrap(t, w.Body.String()).Mode, name)
		}
	}

	w = f.get("status.example.com", "/de/")
	require.Equal(http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(body, "<title>Beta DE</title>")
	assert.Contains(body, `<link rel="canonical" href="http://status.example.com/de/">`)
	b := parseBootstrap(t, body)
	assert.Equal(beta.ID, b.SiteID)
	assert.Empty(b.BasePath)

	w = f.get("status.example.com", "/en/feed.atom")
	require.Equal(http.StatusOK, w.Code)
	assert.Contains(w.Body.String(), `href="http://status.example.com/en/"`)

	w = f.admin(http.MethodGet, "/api/admin/sites", nil)
	sites := decode[[]siteSummary](t, w, http.StatusOK)
	for _, s := range sites {
		assert.Equal(s.ID == beta.ID, s.Landing, s.Route)
	}

	f.putSiteLegal(beta.ID, model.Legal{
		Imprint: model.LegalPage{
			Mode: model.LegalURL,
			URL:  model.Text{"en": "https://beta.example/imprint"},
		},
	})
	w = f.get("status.example.com", "/api/public/legal?lang=en")
	assert.JSONEq(
		`{"imprint": {"mode": "url", "url": "https://beta.example/imprint"}}`,
		w.Body.String(),
		"the login screen links the promoted site's legal pages",
	)

	redirects := func(want map[string]string) {
		t.Helper()
		for url, location := range want {
			host, path, _ := strings.Cut(url, "/")
			w := f.get(host, "/"+path)
			assert.Equal(http.StatusFound, w.Code, url)
			assert.Equal(location, w.Header().Get("Location"), url)
		}
	}

	promote(model.Route{
		Mode: model.RoutePath,
		Slug: "acme",
	})
	redirects(map[string]string{
		"status.example.com/acme":         "http://status.example.com/",
		"sitrep.localhost/acme/incidents": "http://sitrep.localhost/incidents",
	})
	assert.Equal(http.StatusOK, f.get("beta.status.example.com", "/en/").Code)

	promote(model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.gamma.org",
	})
	redirects(map[string]string{
		"status.gamma.org/incidents?page=2": "http://status.example.com/incidents?page=2",
	})
	assert.Equal(http.StatusOK, f.get("status.example.com", "/acme/").Code)
}

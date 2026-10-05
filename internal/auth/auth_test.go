package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

// fakeProvider signs in everyone who posts to /auth/fake/login.
type fakeProvider struct{}

func (fakeProvider) Method() Method  { return MethodCredentials }
func (fakeProvider) Available() bool { return true }
func (fakeProvider) Routes(mux *http.ServeMux, core *Core) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		id := Identity{
			Subject:     "ann",
			DisplayName: "Ann",
			Email:       "ann@example.com",
		}
		if err := core.Login(w, r, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}

	mux.HandleFunc("POST /auth/fake/login", handler)
}

func newCore(t *testing.T, trustProxy bool) (*Core, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "sitrep.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	log := xlog.NewDiscard()
	proxies := httpx.Proxies{All: trustProxy}
	return NewCore(log, db, "fake", fakeProvider{}, time.Hour, proxies), db
}

func post(path, origin string) *http.Request {
	target := "http://status.example.com" + path
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader("{}"))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	return r
}

func login(t *testing.T, core *Core) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	r := post("/auth/fake/login", "http://status.example.com")
	core.Handler().ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	return cookies[0]
}

func TestLoginCookie(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, _ := newCore(t, true)
	c := login(t, core)
	assert.Equal("sitrep_session", c.Name)
	assert.True(c.HttpOnly)
	assert.False(c.Secure)
	assert.Equal(http.SameSiteLaxMode, c.SameSite)
	assert.Equal("/", c.Path)
	assert.Empty(c.Domain)
	assert.WithinDuration(time.Now().Add(time.Hour), c.Expires, time.Minute)
	assert.Len(c.Value, 43, "32 random bytes, base64url-encoded")

	viaTLS := post("/auth/fake/login", "https://status.example.com")
	viaTLS.TLS = &tls.ConnectionState{}
	viaProxy := post("/auth/fake/login", "https://status.example.com")
	viaProxy.Header.Set("X-Forwarded-Proto", "https")
	for name, r := range map[string]*http.Request{
		"tls":           viaTLS,
		"trusted proxy": viaProxy,
	} {
		w := httptest.NewRecorder()
		core.Handler().ServeHTTP(w, r)
		require.Equal(http.StatusNoContent, w.Code, name)
		c := w.Result().Cookies()[0]
		assert.Equal("__Host-sitrep_session", c.Name, name)
		assert.True(c.Secure, name)
	}
}

func TestLoginLogsPendingAccounts(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, db := newCore(t, false)
	var logs bytes.Buffer
	log, err := xlog.New(xlog.AsText(), xlog.WriteTo(&logs))
	require.NoError(err)
	core.Log = log

	grant := func(role model.Role) model.Account {
		acc, err := db.Grant(nil, "fake", "ann@example.com", true, func(a *model.Account) error {
			a.Role = role
			return nil
		})
		require.NoError(err)
		return acc
	}

	ann := grant(model.RoleAdmin)
	login(t, core)
	assert.Contains(logs.String(),
		`msg="bound a pending account on first sign-in" account=`+ann.ID+` subject=ann role=admin`)

	// Ann's email was unverified for a while, and meanwhile provisioned
	// again.
	_, err = db.Grant(nil, "fake", "ann", false, func(a *model.Account) error {
		a.Email = ""
		return nil
	})
	require.NoError(err)
	pending := grant(model.RoleOwner)
	login(t, core)
	assert.Contains(logs.String(),
		`msg="merged a pending account on sign-in" account=`+ann.ID+` pending=`+pending.ID+` subject=ann role=owner`)
}

func TestSessionsStoreOnlyTheHash(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, db := newCore(t, false)
	c := login(t, core)

	_, _, found, err := db.Session([]byte(c.Value))
	require.NoError(err)
	assert.False(found)

	_, _, found, err = db.Session(hashToken(c.Value))
	require.NoError(err)
	assert.True(found)
}

func TestGuard(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, db := newCore(t, false)
	var acc model.Account
	next := func(w http.ResponseWriter, r *http.Request) {
		acc = Account(r.Context())
		w.WriteHeader(http.StatusTeapot)
	}

	ok := core.Guard(http.HandlerFunc(next))
	request := func(c *http.Cookie) *httptest.ResponseRecorder {
		target := "http://status.example.com/api/admin/settings"
		r := httptest.NewRequest(http.MethodGet, target, nil)
		if c != nil {
			r.AddCookie(c)
		}

		w := httptest.NewRecorder()
		ok.ServeHTTP(w, r)
		return w
	}

	w := request(nil)
	assert.Equal(http.StatusUnauthorized, w.Code)
	assert.Contains(w.Body.String(), `"code":"unauthorized"`)

	c := login(t, core)
	assert.Equal(http.StatusTeapot, request(c).Code)
	assert.Equal("ann", acc.Subject, "the account reaches the handler")

	forged := &http.Cookie{
		Name:  c.Name,
		Value: "forged",
	}
	assert.Equal(http.StatusUnauthorized, request(forged).Code)

	httpsName := &http.Cookie{
		Name:  "__Host-sitrep_session",
		Value: c.Value,
	}
	assert.Equal(
		http.StatusUnauthorized,
		request(httpsName).Code,
		"the HTTPS cookie name is not accepted over HTTP",
	)

	session := func(token, account string, expires time.Time) *http.Cookie {
		s := model.Session{
			Account: account,
			Expires: expires,
		}
		require.NoError(db.CreateSession(hashToken(token), s))
		return &http.Cookie{
			Name:  c.Name,
			Value: token,
		}
	}

	expired := session("expired", acc.ID, time.Now().Add(-time.Second))
	assert.Equal(http.StatusUnauthorized, request(expired).Code)

	gone := session("gone", "deleted", time.Now().Add(time.Hour))
	assert.Equal(
		http.StatusUnauthorized,
		request(gone).Code,
		"sessions of deleted accounts are rejected",
	)

	bob, _, _, err := db.SignIn("basic", "bob", "Bob", "")
	require.NoError(err)
	other := session("other", bob.ID, time.Now().Add(time.Hour))
	assert.Equal(
		http.StatusUnauthorized,
		request(other).Code,
		"accounts of another provider are rejected",
	)
}

func TestCSRF(t *testing.T) {
	assert := assert.New(t)

	core, _ := newCore(t, false)
	teapot := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	h := core.CSRF(teapot, "application/json")
	serve := func(method, origin, contentType string) int {
		target := "http://status.example.com:8080/api/admin/x"
		r := httptest.NewRequest(method, target, nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}

		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	const own = "http://status.example.com:8080"
	assert.Equal(http.StatusTeapot, serve(http.MethodGet, "", ""))
	for _, m := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	} {
		assert.Equal(http.StatusTeapot, serve(m, own, "application/json"), m)
		code := serve(
			m,
			"HTTP://Status.Example.com:8080/",
			"application/json; charset=utf-8",
		)
		assert.Equal(http.StatusTeapot, code, m)

		assert.Equal(http.StatusForbidden, serve(m, "", "application/json"), m)
		assert.Equal(http.StatusForbidden, serve(m, "null", "application/json"), m)
		code = serve(m, "http://status.example.com", "application/json")
		assert.Equal(http.StatusForbidden, code, m)
		code = serve(m, "https://status.example.com:8080", "application/json")
		assert.Equal(http.StatusForbidden, code, m)
		code = serve(m, "http://evil.example:8080", "application/json")
		assert.Equal(http.StatusForbidden, code, m)

		assert.Equal(http.StatusUnsupportedMediaType, serve(m, own, ""), m)
		code = serve(m, own, "text/plain")
		assert.Equal(http.StatusUnsupportedMediaType, code, m)
		code = serve(m, own, "application/x-www-form-urlencoded")
		assert.Equal(http.StatusUnsupportedMediaType, code, m)
	}

	form := core.CSRF(teapot, "application/x-www-form-urlencoded")
	target := "http://status.example.com:8080/api/admin/x"
	r := httptest.NewRequest(http.MethodPost, target, nil)
	r.Header.Set("Origin", own)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	form.ServeHTTP(w, r)
	assert.Equal(http.StatusTeapot, w.Code, "other media types where allowed")

	r.Header.Set("Origin", "http://evil.example:8080")
	w = httptest.NewRecorder()
	form.ServeHTTP(w, r)
	assert.Equal(http.StatusForbidden, w.Code)
}

func TestSessionEndpoint(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, _ := newCore(t, false)
	get := func(c *http.Cookie) sessionResponse {
		target := "http://status.example.com/auth/session"
		r := httptest.NewRequest(http.MethodGet, target, nil)
		if c != nil {
			r.AddCookie(c)
		}

		w := httptest.NewRecorder()
		core.Handler().ServeHTTP(w, r)
		require.Equal(http.StatusOK, w.Code)
		assert.Equal("no-store", w.Header().Get("Cache-Control"))
		var res sessionResponse
		require.NoError(json.Unmarshal(w.Body.Bytes(), &res))
		return res
	}

	want := sessionResponse{Provider: sessionProvider{
		ID:        "fake",
		Method:    MethodCredentials,
		Available: true,
		Login:     "email",
	}}
	assert.Equal(want, get(nil))

	user := get(login(t, core)).User
	require.NotNil(user)
	assert.NotEmpty(user.ID)
	user.ID = ""
	assert.Equal(&sessionUser{
		DisplayName: "Ann",
		Email:       "ann@example.com",
		Role:        model.RoleOwner,
		Sites:       map[string]model.Role{},
	}, user, "the first account is the owner")

	core.provider = directoryProvider{}
	assert.Equal("username", get(nil).Provider.Login)
}

// directoryProvider is a fakeProvider that lists its users.
type directoryProvider struct{ fakeProvider }

func (directoryProvider) Users() ([]string, error) {
	return []string{"ann"}, nil
}

func TestLogout(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	core, db := newCore(t, false)
	c := login(t, core)

	target := "http://status.example.com/auth/logout"
	r := httptest.NewRequest(http.MethodPost, target, nil)
	r.AddCookie(c)
	w := httptest.NewRecorder()
	core.Handler().ServeHTTP(w, r)
	assert.Equal(http.StatusForbidden, w.Code, "logout is CSRF-protected")

	r = post("/auth/logout", "http://status.example.com")
	r.AddCookie(c)
	w = httptest.NewRecorder()
	core.Handler().ServeHTTP(w, r)
	assert.Equal(http.StatusSeeOther, w.Code)
	assert.Equal("/admin/?signed-out", w.Header().Get("Location"))
	cleared := w.Result().Cookies()
	require.Len(cleared, 1)
	assert.Equal("sitrep_session", cleared[0].Name)
	assert.Equal(-1, cleared[0].MaxAge)

	_, _, found, err := db.Session(hashToken(c.Value))
	require.NoError(err)
	assert.False(found)
}

func TestReturnPath(t *testing.T) {
	for in, want := range map[string]string{
		"/admin":                     "/admin",
		"/admin/":                    "/admin/",
		"/admin/sites/1?tab=x":       "/admin/sites/1?tab=x",
		"":                           "/admin/",
		"/":                          "/admin/",
		"/administrator":             "/admin/",
		"/admin/../evil":             "/admin/",
		"//evil.example/admin":       "/admin/",
		"https://evil.example/admin": "/admin/",
		`/admin\..\evil`:             "/admin/",
		"admin":                      "/admin/",
	} {
		assert.Equal(t, want, ReturnPath(in), in)
	}
}

func TestNewProvider(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	Register("fake", func(*config.Env, config.Config) Provider {
		return fakeProvider{}
	})

	env := config.NewEnv(func(string) (string, bool) { return "", false })
	assert.Equal(fakeProvider{}, NewProvider(env, config.Config{Auth: "fake"}))
	require.NoError(env.Err())

	assert.Nil(NewProvider(env, config.Config{Auth: "nope"}))
	assert.ErrorContains(env.Err(), "SITREP_AUTH: must be one of fake")
}

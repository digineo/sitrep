package oidc

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/store"
)

func validate(vars map[string]string) (*provider, error) {
	env := config.NewEnv(func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	})
	domains := []string{"status.example.com", "status.example.org"}
	p := New(env, config.Config{BaseDomains: domains})
	return p.(*provider), env.Err()
}

func TestConfig(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	valid := map[string]string{
		"SITREP_OIDC_ISSUER":       "https://idp.example.com/realms/acme",
		"SITREP_OIDC_CLIENT_ID":    "sitrep",
		"SITREP_OIDC_REDIRECT_URL": "https://status.example.com/auth/oidc/callback",
		"SITREP_OIDC_GROUP":        "admins",
	}
	p, err := validate(valid)
	require.NoError(err)
	assert.Equal([]string{"openid", "profile", "email"}, p.oauth.Scopes)
	assert.Equal("groups", p.groupsClaim)
	assert.Equal("admins", p.group)
	assert.Empty(p.deprecated)

	_, err = validate(map[string]string{})
	required := []string{"ISSUER", "CLIENT_ID", "REDIRECT_URL", "GROUP"}
	for _, name := range required {
		assert.ErrorContains(err, "SITREP_OIDC_"+name+": is required")
	}

	for name, value := range map[string]string{
		"SITREP_OIDC_ISSUER":       "idp.example.com",
		"SITREP_OIDC_REDIRECT_URL": "https://status.example.com/callback",
		"SITREP_OIDC_SCOPES":       "",
	} {
		vars := map[string]string{name: value}
		for k, v := range valid {
			if k != name {
				vars[k] = v
			}
		}

		_, err := validate(vars)
		assert.ErrorContains(err, name, value)
	}

	valid["SITREP_OIDC_SCOPES"] = "groups email"
	p, err = validate(valid)
	require.NoError(err)
	want := []string{"openid", "groups", "email"}
	assert.Equal(want, p.oauth.Scopes, "openid is always requested")

	valid["SITREP_OIDC_ADMIN_GROUP"] = "old-admins"
	p, err = validate(valid)
	require.NoError(err)
	assert.Equal("admins", p.group, "the new group variable wins")
	assert.Contains(p.deprecated, "SITREP_OIDC_ADMIN_GROUP is deprecated and ignored")

	delete(valid, "SITREP_OIDC_GROUP")
	p, err = validate(valid)
	require.NoError(err)
	assert.Equal("old-admins", p.group)
	assert.Contains(p.deprecated, "SITREP_OIDC_ADMIN_GROUP is deprecated, use SITREP_OIDC_GROUP")

	valid["SITREP_OIDC_REDIRECT_URL"] = "https://other.example.com/auth/oidc/callback"
	_, err = validate(valid)
	assert.ErrorContains(err, "host must be one of SITREP_BASE_DOMAINS")
}

// mockIdP is an OpenID provider that authorizes every request the test
// approves, signing ID tokens with its own RSA key.
type mockIdP struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu          sync.Mutex
	down        bool                  // answer discovery with 503
	codes       map[string]url.Values // authorization requests by code
	userInfo    map[string]any        // answer of the user info endpoint
	tamper      func(map[string]any)  // changes the claims of ID tokens
	signer      *rsa.PrivateKey       // signs ID tokens instead of key
	tokenParams url.Values            // the last token request
}

// set changes the mock's behavior.
func (m *mockIdP) set(change func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change()
}

func newMockIdP(t *testing.T) *mockIdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	m := &mockIdP{
		t:     t,
		key:   key,
		codes: map[string]url.Values{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", m.discovery)
	mux.HandleFunc("GET /jwks", m.jwks)
	mux.HandleFunc("POST /token", m.token)
	mux.HandleFunc("GET /userinfo", m.userinfo)
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockIdP) discovery(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                m.srv.URL,
		"authorization_endpoint":                m.srv.URL + "/authorize",
		"token_endpoint":                        m.srv.URL + "/token",
		"jwks_uri":                              m.srv.URL + "/jwks",
		"userinfo_endpoint":                     m.srv.URL + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (m *mockIdP) jwks(w http.ResponseWriter, _ *http.Request) {
	enc := base64.RawURLEncoding.EncodeToString
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"alg": "RS256",
		"use": "sig",
		"kid": "test",
		"n":   enc(m.key.N.Bytes()),
		"e":   enc(big.NewInt(int64(m.key.E)).Bytes()),
	}}})
}

// authorize approves an authorization request, as if the user signed in,
// and returns the code.
func (m *mockIdP) authorize(request url.Values) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	code := rand.Text()
	m.codes[code] = request
	return code
}

func (m *mockIdP) token(w http.ResponseWriter, r *http.Request) {
	require.NoError(m.t, r.ParseForm())
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenParams = r.PostForm
	request, ok := m.codes[r.PostForm.Get("code")]
	delete(m.codes, r.PostForm.Get("code"))
	challenge := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	encoded := base64.RawURLEncoding.EncodeToString(challenge[:])
	if !ok || request.Get("code_challenge") != encoded {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}

	claims := map[string]any{
		"iss":            m.srv.URL,
		"aud":            request.Get("client_id"),
		"sub":            "u-1",
		"exp":            time.Now().Add(5 * time.Minute).Unix(),
		"iat":            time.Now().Unix(),
		"nonce":          request.Get("nonce"),
		"name":           "Ann Admin",
		"email":          "ann@example.com",
		"groups":         []string{"staff", "admins"},
		"email_verified": true,
	}
	if m.tamper != nil {
		m.tamper(claims)
	}

	signer := m.key
	if m.signer != nil {
		signer = m.signer
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "access-" + request.Get("nonce"),
		"token_type":   "Bearer",
		"expires_in":   300,
		"id_token":     signJWT(m.t, signer, claims),
	})
}

func (m *mockIdP) userinfo(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer access-") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(m.userInfo)
}

func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	enc := func(v any) string {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(data)
	}

	header := map[string]string{
		"alg": "RS256",
		"kid": "test",
		"typ": "JWT",
	}
	payload := enc(header) + "." + enc(claims)
	digest := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return payload + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// lockedBuffer collects log output written from several goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type fixture struct {
	t       *testing.T
	idp     *mockIdP
	core    *auth.Core
	handler http.Handler
	log     *lockedBuffer
}

// newFixture serves the provider's routes for the base domains
// status.example.com, with the callback, and status.example.org. Discovery
// has succeeded unless the identity provider is down. The vars are pairs
// of names and values; an empty value unsets the variable.
func newFixture(t *testing.T, down bool, vars ...string) *fixture {
	require := require.New(t)

	idp := newMockIdP(t)
	idp.down = down
	env := map[string]string{
		"SITREP_OIDC_ISSUER":       idp.srv.URL,
		"SITREP_OIDC_CLIENT_ID":    "sitrep",
		"SITREP_OIDC_REDIRECT_URL": "https://status.example.com/auth/oidc/callback",
		"SITREP_OIDC_GROUP":        "admins",
	}
	for i := 0; i < len(vars); i += 2 {
		if vars[i+1] == "" {
			delete(env, vars[i])
		} else {
			env[vars[i]] = vars[i+1]
		}
	}

	p, err := validate(env)
	require.NoError(err)
	p.retry = 10 * time.Millisecond
	p.trustProxy = true

	db, err := store.Open(filepath.Join(t.TempDir(), "sitrep.db"))
	require.NoError(err)
	t.Cleanup(func() { _ = db.Close() })

	log := &lockedBuffer{}
	logger, err := xlog.New(xlog.AsText(), xlog.WriteTo(log))
	require.NoError(err)

	core := auth.NewCore(logger, db, "oidc", p, time.Hour, true)
	f := &fixture{
		t:       t,
		idp:     idp,
		core:    core,
		handler: core.Handler(),
		log:     log,
	}
	if !down {
		require.Eventually(p.Available, 5*time.Second, 5*time.Millisecond)
	}
	return f
}

// get requests path on host over HTTPS behind a trusted proxy.
func (f *fixture) get(host, path string, cookies ...*http.Cookie) *http.Response {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = host
	r.Header.Set("X-Forwarded-Proto", "https")
	for _, c := range cookies {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w.Result()
}

func cookie(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// login starts a sign-in and returns the flow cookie and the authorization
// request.
func (f *fixture) login(ret string) (*http.Cookie, url.Values) {
	require := require.New(f.t)

	path := "/auth/oidc/login?return=" + url.QueryEscape(ret)
	res := f.get("status.example.com", path)
	require.Equal(http.StatusFound, res.StatusCode)

	target, err := url.Parse(res.Header.Get("Location"))
	require.NoError(err)
	endpoint := target.Scheme + "://" + target.Host + target.Path
	require.Equal(f.idp.srv.URL+"/authorize", endpoint)

	flow := cookie(res, flowCookie)
	require.NotNil(flow)
	return flow, target.Query()
}

// callback completes a sign-in with query and returns the response. It
// checks that the flow cookie is cleared.
func (f *fixture) callback(
	query url.Values,
	cookies ...*http.Cookie,
) *http.Response {
	path := "/auth/oidc/callback?" + query.Encode()
	res := f.get("status.example.com", path, cookies...)
	cleared := cookie(res, flowCookie)
	if assert.NotNil(f.t, cleared, "the flow cookie is cleared") {
		assert.Negative(f.t, cleared.MaxAge)
		assert.Equal(f.t, "/auth/oidc/", cleared.Path)
	}
	return res
}

// signIn runs the whole flow and returns the callback's response.
func (f *fixture) signIn() *http.Response {
	flow, request := f.login("/admin/sites?x=1")
	code := f.idp.authorize(request)
	query := url.Values{
		"code":  {code},
		"state": {request.Get("state")},
	}
	return f.callback(query, flow)
}

func assertLoginError(t *testing.T, res *http.Response, location string) {
	t.Helper()
	assert.Equal(t, http.StatusSeeOther, res.StatusCode)
	assert.Equal(t, location, res.Header.Get("Location"))
	assert.Nil(t, cookie(res, "__Host-sitrep_session"), "no session")
}

func TestSignIn(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, false)
	flow, request := f.login("/admin/sites?x=1")
	assert.Equal("/auth/oidc/", flow.Path)
	assert.True(flow.HttpOnly)
	assert.True(flow.Secure)
	assert.Equal(http.SameSiteLaxMode, flow.SameSite)
	assert.Equal(600, flow.MaxAge)

	assert.Equal("code", request.Get("response_type"))
	assert.Equal("sitrep", request.Get("client_id"))
	redirect := request.Get("redirect_uri")
	assert.Equal("https://status.example.com/auth/oidc/callback", redirect)
	assert.Equal("openid profile email", request.Get("scope"))
	assert.Equal("S256", request.Get("code_challenge_method"))
	assert.NotEmpty(request.Get("state"))
	assert.NotEmpty(request.Get("nonce"))
	assert.NotEqual(request.Get("state"), request.Get("nonce"))

	code := f.idp.authorize(request)
	query := url.Values{
		"code":  {code},
		"state": {request.Get("state")},
	}
	res := f.callback(query, flow)
	assert.Equal(http.StatusSeeOther, res.StatusCode)
	assert.Equal("/admin/sites?x=1", res.Header.Get("Location"))
	f.idp.set(func() {
		clientID := f.idp.tokenParams.Get("client_id")
		assert.Equal("sitrep", clientID, "public clients send their ID")
	})

	session := cookie(res, "__Host-sitrep_session")
	require.NotNil(session)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.AddCookie(session)
	acc, ok, err := f.core.User(r)
	require.NoError(err)
	require.True(ok)
	assert.Equal("u-1", acc.Subject)
	assert.Equal("Ann Admin", acc.DisplayName)
	assert.Equal("ann@example.com", acc.Email)

	replay := f.callback(query)
	assertLoginError(t, replay, "/admin/?login-error=idp_error")
}

func TestSignInDisplayName(t *testing.T) {
	f := newFixture(t, false)
	for name, tt := range map[string]struct {
		claims map[string]any
		want   string
	}{
		"name":               {map[string]any{"name": " Ann "}, "Ann"},
		"preferred_username": {map[string]any{"name": "", "preferred_username": "ann"}, "ann"},
		"subject":            {map[string]any{"name": nil, "email": nil}, "u-1"},
	} {
		f.idp.set(func() {
			f.idp.tamper = func(c map[string]any) {
				for k, v := range tt.claims {
					if v == nil {
						delete(c, k)
					} else {
						c[k] = v
					}
				}
			}
		})

		res := f.signIn()
		require.Equal(t, "/admin/sites?x=1", res.Header.Get("Location"), name)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		r.AddCookie(cookie(res, "__Host-sitrep_session"))
		acc, _, err := f.core.User(r)
		require.NoError(t, err)
		assert.Equal(t, tt.want, acc.DisplayName, name)
	}
}

func TestSignInEmailVerified(t *testing.T) {
	f := newFixture(t, false)
	for name, tt := range map[string]struct {
		verified any // nil removes the claim
		want     string
	}{
		"verified":   {true, "ann@example.com"},
		"unverified": {false, ""},
		"string":     {"true", ""},
		"missing":    {nil, ""},
	} {
		f.idp.set(func() {
			f.idp.tamper = func(c map[string]any) {
				if tt.verified == nil {
					delete(c, "email_verified")
				} else {
					c["email_verified"] = tt.verified
				}
			}
		})

		res := f.signIn()
		require.Equal(t, "/admin/sites?x=1", res.Header.Get("Location"), name)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		r.AddCookie(cookie(res, "__Host-sitrep_session"))
		acc, _, err := f.core.User(r)
		require.NoError(t, err)
		assert.Equal(t, tt.want, acc.Email, name)
	}
}

func TestSignInGroups(t *testing.T) {
	tests := []struct {
		name     string
		claim    any // nil removes the claim from the ID token
		userInfo map[string]any
		want     string // error code, or "" for success
	}{
		{"array", []string{"staff", "admins"}, nil, ""},
		{"single string", "admins", nil, ""},
		{"other groups", []string{"staff", "admins-old"}, nil, "not_member"},
		{"other single string", "staff", nil, "not_member"},
		{"empty array", []string{}, nil, "not_member"},
		{"wrong type", map[string]any{"admins": true}, nil, "not_member"},
		{"from user info", nil, map[string]any{"sub": "u-1", "groups": []string{"admins"}}, ""},
		{"not in user info either", nil, map[string]any{"sub": "u-1"}, "not_member"},
		{"user info of another subject", nil, map[string]any{"sub": "u-2", "groups": []string{"admins"}}, "idp_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			f := newFixture(t, false)
			f.idp.set(func() {
				f.idp.userInfo = tt.userInfo
				f.idp.tamper = func(c map[string]any) {
					c["email"] = "secret-mail@example.com"
					if tt.claim == nil {
						delete(c, "groups")
					} else {
						c["groups"] = tt.claim
					}
				}
			})

			res := f.signIn()
			if tt.want == "" {
				assert.Equal("/admin/sites?x=1", res.Header.Get("Location"))
				assert.NotNil(cookie(res, "__Host-sitrep_session"))
				return
			}

			assertLoginError(t, res, "/admin/sites?login-error="+tt.want+"&x=1")
			if tt.want == "not_member" {
				assert.Contains(f.log.String(), "u-1", "denials name the subject")
				assert.NotContains(
					f.log.String(),
					"secret-mail",
					"but not the email",
				)
			}
		})
	}
}

func TestSignInGroupsClaim(t *testing.T) {
	f := newFixture(
		t,
		false,
		"SITREP_OIDC_GROUPS_CLAIM", "roles",
		"SITREP_OIDC_GROUP", "sitrep-admin",
	)
	f.idp.set(func() {
		f.idp.tamper = func(c map[string]any) {
			c["roles"] = []string{"sitrep-admin"}
		}
	})

	assert.Equal(t, "/admin/sites?x=1", f.signIn().Header.Get("Location"))

	f.idp.set(func() {
		f.idp.tamper = func(c map[string]any) {
			c["roles"] = []string{"admins"}
		}
	})

	assertLoginError(t, f.signIn(), "/admin/sites?login-error=not_member&x=1")
}

func TestSignInDeprecatedGroup(t *testing.T) {
	f := newFixture(
		t,
		false,
		"SITREP_OIDC_GROUP", "",
		"SITREP_OIDC_ADMIN_GROUP", "admins",
	)
	assert.Contains(t, f.log.String(), "SITREP_OIDC_ADMIN_GROUP is deprecated")
	assert.Equal(t, "/admin/sites?x=1", f.signIn().Header.Get("Location"))
}

func TestSignInRejectsInvalidTokens(t *testing.T) {
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for name, tamper := range map[string]func(map[string]any){
		"wrong issuer":   func(c map[string]any) { c["iss"] = "https://evil.example.com" },
		"wrong audience": func(c map[string]any) { c["aud"] = "someone-else" },
		"expired":        func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"wrong nonce":    func(c map[string]any) { c["nonce"] = "replayed" },
		"no nonce":       func(c map[string]any) { delete(c, "nonce") },
		"wrong key":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, false)
			f.idp.set(func() {
				f.idp.tamper = tamper
				if tamper == nil {
					f.idp.signer = other
				}
			})
			res := f.signIn()
			assertLoginError(t, res, "/admin/sites?login-error=idp_error&x=1")
		})
	}
}

func TestSignInChecksTheFlow(t *testing.T) {
	f := newFixture(t, false)
	flow, request := f.login("/admin/")
	state := request.Get("state")

	code := f.idp.authorize(request)
	query := url.Values{
		"code":  {code},
		"state": {"forged"},
	}
	res := f.callback(query, flow)
	assertLoginError(t, res, "/admin/?login-error=idp_error")

	code = f.idp.authorize(request)
	res = f.callback(url.Values{
		"code":  {code},
		"state": {state},
	})
	assertLoginError(t, res, "/admin/?login-error=idp_error")

	// The PKCE verifier in the flow cookie must match the challenge.
	var fl struct{ State, Nonce, Verifier, Return string }
	data, err := base64.RawURLEncoding.DecodeString(flow.Value)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &fl))
	fl.Verifier = strings.Repeat("x", 43)
	data, err = json.Marshal(fl)
	require.NoError(t, err)
	code = f.idp.authorize(request)
	forged := &http.Cookie{
		Name:  flowCookie,
		Value: base64.RawURLEncoding.EncodeToString(data),
	}
	query = url.Values{
		"code":  {code},
		"state": {state},
	}
	res = f.callback(query, forged)
	assertLoginError(t, res, "/admin/?login-error=idp_error")

	garbage := &http.Cookie{
		Name:  flowCookie,
		Value: "garbage",
	}
	res = f.callback(query, garbage)
	assertLoginError(t, res, "/admin/?login-error=idp_error")
}

func TestSignInIdPErrors(t *testing.T) {
	f := newFixture(t, false)
	for idpError, code := range map[string]string{
		"access_denied":  "denied",
		"server_error":   "idp_error",
		"login_required": "idp_error",
	} {
		flow, request := f.login("/admin/incidents")
		query := url.Values{
			"error": {idpError},
			"state": {request.Get("state")},
		}
		res := f.callback(query, flow)
		assertLoginError(t, res, "/admin/incidents?login-error="+code)
	}
}

func TestReturnPathStaysInTheConsole(t *testing.T) {
	f := newFixture(t, false)
	for _, ret := range []string{
		"https://evil.example.com/admin/",
		"//evil.example.com/admin/",
		"/elsewhere",
		"",
	} {
		flow, request := f.login(ret)
		code := f.idp.authorize(request)
		query := url.Values{
			"code":  {code},
			"state": {request.Get("state")},
		}
		res := f.callback(query, flow)
		assert.Equal(t, "/admin/", res.Header.Get("Location"), ret)
	}
}

func TestLoginMovesToTheCallbackHost(t *testing.T) {
	f := newFixture(t, false)
	path := "/auth/oidc/login?return=%2Fadmin%2Fsettings"
	res := f.get("status.example.org", path)
	assert.Equal(t, http.StatusFound, res.StatusCode)
	want := "https://status.example.com/auth/oidc/login?return=%2Fadmin%2Fsettings"
	assert.Equal(t, want, res.Header.Get("Location"))
	assert.Nil(t, cookie(res, flowCookie))
}

func TestDiscoveryRetries(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, true)
	path := "/auth/oidc/login?return=%2Fadmin%2Fsettings"
	res := f.get("status.example.com", path)
	assertLoginError(t, res, "/admin/settings?login-error=unavailable")
	assert.Nil(cookie(res, flowCookie))

	var session struct{ Provider struct{ Available bool } }
	body := f.get("status.example.com", "/auth/session").Body
	require.NoError(json.NewDecoder(body).Decode(&session))
	assert.False(session.Provider.Available)

	failedTwice := func() bool {
		return strings.Count(f.log.String(), "OIDC discovery failed") >= 2
	}

	require.Eventually(failedTwice, 5*time.Second, 5*time.Millisecond)

	f.idp.set(func() { f.idp.down = false })
	available := func() bool {
		body := f.get("status.example.com", "/auth/session").Body
		require.NoError(json.NewDecoder(body).Decode(&session))
		return session.Provider.Available
	}

	require.Eventually(available, 5*time.Second, 5*time.Millisecond)
	res = f.get("status.example.com", "/auth/oidc/login")
	assert.Equal(http.StatusFound, res.StatusCode)
}

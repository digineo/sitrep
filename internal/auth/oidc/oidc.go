// Package oidc signs admins in with OpenID Connect: the authorization code
// flow with PKCE, state and nonce. Only members of the sign-in group may
// sign in.
package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/digineo/xlog"
	"golang.org/x/oauth2"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
)

func init() {
	auth.Register("oidc", New)
}

const (
	// flowCookie keeps state, nonce, PKCE verifier and return path between
	// login and callback.
	flowCookie = "sitrep_oidc"
	flowTTL    = 10 * time.Minute
	maxRetry   = time.Minute // longest wait between discovery attempts
)

// Error codes the login screen shows a notice for.
const (
	errDenied      = "denied"
	errNotMember   = "not_member"
	errIdP         = "idp_error"
	errUnavailable = "unavailable"
)

type provider struct {
	issuer      string
	oauth       oauth2.Config // without endpoint, which discovery finds
	redirect    *url.URL
	groupsClaim string
	group       string
	deprecated  string // warning about the group variable, if any
	trustProxy  bool
	client      *http.Client  // for every request to the identity provider
	retry       time.Duration // first wait between discovery attempts

	idp atomic.Pointer[oidc.Provider] // nil until discovery succeeds
}

// New reads the SITREP_OIDC_* variables.
func New(env *config.Env, cfg config.Config) auth.Provider {
	const redirect = "SITREP_OIDC_REDIRECT_URL"
	p := &provider{
		trustProxy: cfg.TrustProxy,
		client:     &http.Client{Timeout: 10 * time.Second},
		retry:      time.Second,
	}
	if u := env.URL("SITREP_OIDC_ISSUER", true); u != nil {
		p.issuer = u.String()
	}

	p.oauth.ClientID = env.Required("SITREP_OIDC_CLIENT_ID")
	p.oauth.ClientSecret = env.Secret("SITREP_OIDC_CLIENT_SECRET")
	if u := env.URL(redirect, true); u != nil {
		if u.Path != "/auth/oidc/callback" {
			env.Errorf(redirect, "path must be /auth/oidc/callback")
		}

		if !slices.Contains(cfg.BaseDomains, u.Hostname()) {
			env.Errorf(redirect, "host must be one of SITREP_BASE_DOMAINS")
		}

		p.redirect = u
		p.oauth.RedirectURL = u.String()
	}

	scopes := env.String("SITREP_OIDC_SCOPES", "openid profile email")
	p.oauth.Scopes = strings.Fields(scopes)
	if !slices.Contains(p.oauth.Scopes, oidc.ScopeOpenID) {
		p.oauth.Scopes = append([]string{oidc.ScopeOpenID}, p.oauth.Scopes...)
	}

	p.groupsClaim = env.String("SITREP_OIDC_GROUPS_CLAIM", "groups")
	p.group, p.deprecated = group(env)
	return p
}

// group reads the sign-in group from SITREP_OIDC_GROUP, or from its
// deprecated predecessor, and returns a warning if that one is set.
func group(env *config.Env) (string, string) {
	const name, legacy = "SITREP_OIDC_GROUP", "SITREP_OIDC_ADMIN_GROUP"
	g := env.String(name, "")
	old := env.String(legacy, "")
	switch {
	case g == "" && old == "":
		env.Errorf(name, "is required")
	case g == "":
		return old, legacy + " is deprecated, use " + name
	case old != "":
		return g, legacy + " is deprecated and ignored, since " + name + " is set"
	}
	return g, ""
}

func (p *provider) Method() auth.Method { return auth.MethodRedirect }

// Available reports whether discovery has succeeded.
func (p *provider) Available() bool { return p.idp.Load() != nil }

// Routes registers the login and callback routes and starts discovery.
func (p *provider) Routes(mux *http.ServeMux, core *auth.Core) {
	if p.deprecated != "" {
		core.Log.Warn(p.deprecated)
	}

	go p.discover(core.Log)
	callback := func(w http.ResponseWriter, r *http.Request) {
		p.callback(w, r, core)
	}
	mux.HandleFunc("GET /auth/oidc/login", p.login)
	mux.HandleFunc("GET /auth/oidc/callback", callback)
}

// discover fetches the identity provider's configuration. It retries with
// exponential backoff until it succeeds.
func (p *provider) discover(log xlog.Logger) {
	ctx := oidc.ClientContext(context.Background(), p.client)
	for wait := p.retry; ; wait = min(2*wait, maxRetry) {
		idp, err := oidc.NewProvider(ctx, p.issuer)
		if err == nil {
			p.idp.Store(idp)
			log.Info("OIDC discovery succeeded",
				slog.String("issuer", p.issuer))
			return
		}

		log.Warn("OIDC discovery failed, sign-in is unavailable until it succeeds",
			xlog.Error(err),
			slog.Duration("retry", wait))
		time.Sleep(wait)
	}
}

// config returns the OAuth 2.0 configuration with the discovered endpoint.
// Public clients send their ID in the request body.
func (p *provider) config(idp *oidc.Provider) *oauth2.Config {
	c := p.oauth
	c.Endpoint = idp.Endpoint()
	if c.ClientSecret == "" {
		c.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	return &c
}

type flow struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Return   string `json:"return"`
}

func (p *provider) cookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     flowCookie,
		Value:    value,
		Path:     "/auth/oidc/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   httpx.Effective(r, p.trustProxy).Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	}
}

// login sends the browser to the identity provider. Cookies are bound to
// their host, so a login on another base domain than the callback's first
// moves to the callback's host.
func (p *provider) login(w http.ResponseWriter, r *http.Request) {
	ret := auth.ReturnPath(r.URL.Query().Get("return"))
	if httpx.Effective(r, p.trustProxy).Host != p.redirect.Hostname() {
		u := url.URL{
			Scheme:   p.redirect.Scheme,
			Host:     p.redirect.Host,
			Path:     r.URL.Path,
			RawQuery: url.Values{"return": {ret}}.Encode(),
		}
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}

	idp := p.idp.Load()
	if idp == nil {
		fail(w, r, ret, errUnavailable)
		return
	}

	f := flow{
		State:    rand.Text(),
		Nonce:    rand.Text(),
		Verifier: oauth2.GenerateVerifier(),
		Return:   ret,
	}
	value, _ := json.Marshal(f)
	encoded := base64.RawURLEncoding.EncodeToString(value)
	http.SetCookie(w, p.cookie(r, encoded, int(flowTTL.Seconds())))
	pkce := oauth2.S256ChallengeOption(f.Verifier)
	target := p.config(idp).AuthCodeURL(f.State, oidc.Nonce(f.Nonce), pkce)
	http.Redirect(w, r, target, http.StatusFound)
}

// readFlow returns the flow started by login, if the request carries one.
func readFlow(r *http.Request) (flow, bool) {
	var f flow
	c, err := r.Cookie(flowCookie)
	if err != nil {
		return f, false
	}

	data, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || json.Unmarshal(data, &f) != nil || f.State == "" {
		return flow{}, false
	}

	f.Return = auth.ReturnPath(f.Return)
	return f, true
}

// callback completes the flow: it checks the provider's answer and the
// sign-in group, then starts a session. Every failure returns to the login
// screen with an error code.
func (p *provider) callback(
	w http.ResponseWriter,
	r *http.Request,
	core *auth.Core,
) {
	f, ok := readFlow(r)
	http.SetCookie(w, p.cookie(r, "", -1))
	ret := "/admin/"
	if ok {
		ret = f.Return
	}

	id, code := p.identify(r, f, ok, core.Log)
	if code == "" {
		if err := core.Login(w, r, id); err != nil {
			core.Log.Error("starting a session failed",
				xlog.Error(err))
			code = errUnavailable
		}
	}
	if code != "" {
		fail(w, r, ret, code)
		return
	}

	http.Redirect(w, r, ret, http.StatusSeeOther)
}

// identify returns the identity of an admin, with the email only if the
// provider verified it, or the error code to show.
func (p *provider) identify(
	r *http.Request,
	f flow,
	ok bool,
	log xlog.Logger,
) (auth.Identity, string) {
	q := r.URL.Query()
	switch {
	case q.Get("error") == "access_denied":
		return auth.Identity{}, errDenied
	case q.Get("error") != "":
		log.Warn("the identity provider refused the sign-in",
			slog.String("error", q.Get("error")),
			slog.String("description", q.Get("error_description")))
		return auth.Identity{}, errIdP
	case !ok || q.Get("state") != f.State:
		log.Info("OIDC callback without the state of a sign-in started here")
		return auth.Identity{}, errIdP
	}

	idp := p.idp.Load()
	if idp == nil {
		return auth.Identity{}, errUnavailable
	}

	// oauth2 and go-oidc each look up their HTTP client in the context.
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, p.client)
	ctx = oidc.ClientContext(ctx, p.client)
	pkce := oauth2.VerifierOption(f.Verifier)
	token, err := p.config(idp).Exchange(ctx, q.Get("code"), pkce)
	if err != nil {
		log.Warn("exchanging the OIDC authorization code failed",
			xlog.Error(err))
		return auth.Identity{}, errIdP
	}

	raw, _ := token.Extra("id_token").(string)
	verifier := idp.Verifier(&oidc.Config{ClientID: p.oauth.ClientID})
	idToken, err := verifier.Verify(ctx, raw)
	if err != nil {
		log.Warn("the OIDC ID token is invalid",
			xlog.Error(err))
		return auth.Identity{}, errIdP
	}

	if idToken.Nonce != f.Nonce {
		log.Warn("the OIDC ID token has the wrong nonce",
			slog.String("subject", idToken.Subject))
		return auth.Identity{}, errIdP
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		log.Warn("reading the OIDC ID token's claims failed",
			xlog.Error(err))
		return auth.Identity{}, errIdP
	}

	if _, found := claims[p.groupsClaim]; !found {
		err := p.addUserInfo(ctx, idp, token, idToken.Subject, claims)
		if err != nil {
			log.Warn("fetching the OIDC user info failed",
				xlog.Error(err))
			return auth.Identity{}, errIdP
		}
	}

	if !member(claims[p.groupsClaim], p.group) {
		log.Info("denied sign-in: not a member of the sign-in group",
			slog.String("subject", idToken.Subject))
		return auth.Identity{}, errNotMember
	}

	str := func(name string) string {
		s, _ := claims[name].(string)
		return strings.TrimSpace(s)
	}

	id := auth.Identity{
		Subject:     idToken.Subject,
		DisplayName: str("name"),
	}
	if verified, _ := claims["email_verified"].(bool); verified {
		id.Email = str("email")
	}
	if id.DisplayName == "" {
		id.DisplayName = str("preferred_username")
	}
	if id.DisplayName == "" {
		id.DisplayName = id.Subject
	}
	return id, ""
}

// addUserInfo adds the claims of the user info endpoint that the ID token
// lacks.
func (p *provider) addUserInfo(
	ctx context.Context,
	idp *oidc.Provider,
	token *oauth2.Token,
	subject string,
	claims map[string]any,
) error {
	info, err := idp.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return err
	}

	if info.Subject != subject {
		return errors.New(
			"the user info belongs to another subject than the ID token",
		)
	}

	var more map[string]any
	if err := info.Claims(&more); err != nil {
		return err
	}

	for name, v := range more {
		if _, found := claims[name]; !found {
			claims[name] = v
		}
	}
	return nil
}

// member reports whether groups, a string or an array of strings, contains
// group.
func member(groups any, group string) bool {
	switch groups := groups.(type) {
	case string:
		return groups == group
	case []any:
		return slices.Contains(groups, any(group))
	}
	return false
}

// fail returns to the login screen at ret with an error code.
func fail(w http.ResponseWriter, r *http.Request, ret, code string) {
	u, _ := url.Parse(ret)
	q := u.Query()
	q.Set("login-error", code)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"time"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

// Core manages sessions for the active provider.
type Core struct {
	Log xlog.Logger

	db         *store.DB
	providerID string
	provider   Provider
	ttl        time.Duration
	trustProxy bool
}

// NewCore returns a Core for the provider registered as providerID.
func NewCore(
	log xlog.Logger,
	db *store.DB,
	providerID string,
	provider Provider,
	ttl time.Duration,
	trustProxy bool,
) *Core {
	return &Core{
		Log:        log,
		db:         db,
		providerID: providerID,
		provider:   provider,
		ttl:        ttl,
		trustProxy: trustProxy,
	}
}

// Handler serves /auth/: the session endpoint, logout and the provider's
// routes, all behind CSRF protection.
func (c *Core) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/session", c.session)
	mux.HandleFunc("POST /auth/logout", c.logout)
	c.provider.Routes(mux, c)
	return c.CSRF(mux)
}

// cookieName returns the session cookie's name. HTTPS requests use the
// __Host- prefix, which browsers only accept for secure host-only cookies.
func cookieName(https bool) string {
	if https {
		return "__Host-sitrep_session"
	}
	return "sitrep_session"
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func (c *Core) https(r *http.Request) bool {
	return httpx.Effective(r, c.trustProxy).Scheme == "https"
}

// Login starts a session for id and sets the session cookie.
func (c *Core) Login(w http.ResponseWriter, r *http.Request, id Identity) error {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(c.ttl)
	err := c.db.CreateSession(hashToken(token), model.Session{
		Provider:    c.providerID,
		Subject:     id.Subject,
		DisplayName: id.DisplayName,
		Email:       id.Email,
		Expires:     expires,
	})
	if err != nil {
		return err
	}

	https := c.https(r)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName(https),
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   https,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// User returns the request's session, if it has a valid one for the active
// provider.
func (c *Core) User(r *http.Request) (model.Session, bool, error) {
	cookie, err := r.Cookie(cookieName(c.https(r)))
	if err != nil {
		return model.Session{}, false, nil
	}

	s, found, err := c.db.Session(hashToken(cookie.Value))
	if err != nil || !found || !s.Expires.After(time.Now()) ||
		s.Provider != c.providerID {
		return model.Session{}, false, err
	}
	return s, true, nil
}

// Guard answers requests without a valid session with 401.
func (c *Core) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok, err := c.User(r)
		switch {
		case err != nil:
			httpx.WriteError(w, r, c.Log, err)
		case !ok:
			e := apierr.New(http.StatusUnauthorized, apierr.Unauthorized)
			httpx.WriteError(w, r, c.Log, e)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// CSRF rejects requests with unsafe methods unless their Origin is the
// request's own origin and their body is JSON.
func (c *Core) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		origin, ok := model.NormalizeOrigin(r.Header.Get("Origin"))
		if !ok || origin != httpx.Effective(r, c.trustProxy).Origin {
			e := apierr.New(http.StatusForbidden, apierr.Forbidden)
			httpx.WriteError(w, r, c.Log, e)
			return
		}

		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			e := apierr.New(
				http.StatusUnsupportedMediaType,
				apierr.UnsupportedMediaType,
			)
			httpx.WriteError(w, r, c.Log, e)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type sessionResponse struct {
	User     *sessionUser    `json:"user"`
	Provider sessionProvider `json:"provider"`
}

type sessionUser struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

type sessionProvider struct {
	ID        string `json:"id"`
	Method    Method `json:"method"`
	Available bool   `json:"available"`
}

// session reports the signed-in admin, or null, and how to sign in.
func (c *Core) session(w http.ResponseWriter, r *http.Request) {
	s, ok, err := c.User(r)
	if err != nil {
		httpx.WriteError(w, r, c.Log, err)
		return
	}

	res := sessionResponse{Provider: sessionProvider{
		ID:        c.providerID,
		Method:    c.provider.Method(),
		Available: c.provider.Available(),
	}}
	if ok {
		res.User = &sessionUser{
			DisplayName: s.DisplayName,
			Email:       s.Email,
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, res)
}

// logout deletes the session, clears the cookie and shows the login screen
// with a notice.
func (c *Core) logout(w http.ResponseWriter, r *http.Request) {
	https := c.https(r)
	if cookie, err := r.Cookie(cookieName(https)); err == nil {
		if err := c.db.DeleteSession(hashToken(cookie.Value)); err != nil {
			httpx.WriteError(w, r, c.Log, err)
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName(https),
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   https,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin/?signed-out", http.StatusSeeOther)
}

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"mime"
	"net/http"
	"slices"
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
	return c.CSRF(mux, "application/json")
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

// Login signs in id's account, creating it on first sign-in, and starts a
// session with the session cookie.
func (c *Core) Login(w http.ResponseWriter, r *http.Request, id Identity) error {
	acc, pending, created, err := c.db.SignIn(
		c.providerID,
		id.Subject,
		id.DisplayName,
		id.Email,
	)
	if err != nil {
		return err
	}

	switch {
	case created:
		c.Log.Info("created an account on first sign-in",
			slog.String("account", acc.ID),
			slog.String("subject", acc.Subject),
			slog.String("role", string(acc.Role)))
	case pending == acc.ID:
		c.Log.Info("bound a pending account on first sign-in",
			slog.String("account", acc.ID),
			slog.String("subject", acc.Subject),
			slog.String("role", string(acc.Role)),
			slog.Any("sites", acc.Sites))
	case pending != "":
		c.Log.Info("merged a pending account on sign-in",
			slog.String("account", acc.ID),
			slog.String("pending", pending),
			slog.String("subject", acc.Subject),
			slog.String("role", string(acc.Role)),
			slog.Any("sites", acc.Sites))
	}

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(c.ttl)
	err = c.db.CreateSession(hashToken(token), model.Session{
		Account: acc.ID,
		Expires: expires,
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

// User returns the account of the request's session, if it has a
// valid one for an account of the active provider that its directory, if
// any, still lists.
func (c *Core) User(r *http.Request) (model.Account, bool, error) {
	cookie, err := r.Cookie(cookieName(c.https(r)))
	if err != nil {
		return model.Account{}, false, nil
	}

	s, acc, found, err := c.db.Session(hashToken(cookie.Value))
	if err != nil || !found || !s.Expires.After(time.Now()) ||
		acc == nil || acc.Provider != c.providerID {
		return model.Account{}, false, err
	}

	if d := c.Directory(); d != nil {
		users, err := d.Users()
		if err != nil {
			c.Log.Error("reloading the auth provider's users failed, using the previous ones",
				xlog.Error(err))
		}
		if !slices.Contains(users, acc.Subject) {
			return model.Account{}, false, nil
		}
	}
	return *acc, true, nil
}

// Directory returns the provider's directory, or nil if it has none.
func (c *Core) Directory() Directory {
	d, _ := c.provider.(Directory)
	return d
}

type accountKey struct{}

// Guard answers requests without a valid session with 401. Others reach
// next with the session's account in their context.
func (c *Core) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acc, ok, err := c.User(r)
		switch {
		case err != nil:
			httpx.WriteError(w, r, c.Log, err)
		case !ok:
			e := apierr.New(http.StatusUnauthorized, apierr.Unauthorized)
			httpx.WriteError(w, r, c.Log, e)
		default:
			ctx := context.WithValue(r.Context(), accountKey{}, acc)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// Account returns the account of a request that passed the guard.
func Account(ctx context.Context) model.Account {
	acc, _ := ctx.Value(accountKey{}).(model.Account)
	return acc
}

// CSRF rejects requests with unsafe methods unless their Origin is the
// request's own origin and their body has one of the media types.
func (c *Core) CSRF(next http.Handler, mediaTypes ...string) http.Handler {
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
		if err != nil || !slices.Contains(mediaTypes, mt) {
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
	ID          string                `json:"id"`
	DisplayName string                `json:"displayName"`
	Email       string                `json:"email,omitempty"`
	Role        model.Role            `json:"role"`
	Sites       map[string]model.Role `json:"sites"`
}

type sessionProvider struct {
	ID        string `json:"id"`
	Method    Method `json:"method"`
	Available bool   `json:"available"`
	// Login says how accounts are named: "username" for providers with a
	// directory, else "email".
	Login string `json:"login"`
}

// session reports the signed-in admin with their roles, or null, and how
// to sign in.
func (c *Core) session(w http.ResponseWriter, r *http.Request) {
	acc, ok, err := c.User(r)
	if err != nil {
		httpx.WriteError(w, r, c.Log, err)
		return
	}

	res := sessionResponse{Provider: sessionProvider{
		ID:        c.providerID,
		Method:    c.provider.Method(),
		Available: c.provider.Available(),
		Login:     "email",
	}}
	if c.Directory() != nil {
		res.Provider.Login = "username"
	}
	if ok {
		res.User = &sessionUser{
			ID:          acc.ID,
			DisplayName: acc.DisplayName,
			Email:       acc.Email,
			Role:        acc.Role,
			Sites:       acc.Sites,
		}
		if res.User.Sites == nil {
			res.User.Sites = map[string]model.Role{}
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

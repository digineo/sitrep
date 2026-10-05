// Package auth owns admin sessions, the admin API guard, CSRF protection and
// logout. Auth providers authenticate admins and hand their identity to the
// Core, which starts the session.
//
// A provider lives in its own package, registers itself with Register in
// an init function, and is linked into the binary by a blank import in
// cmd/sitrep.
package auth

import (
	"net/http"
	"net/mail"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/digineo/sitrep/internal/config"
)

// Method tells the login screen how to sign in.
type Method string

// Login methods.
const (
	// MethodRedirect sends the browser to /auth/{id}/login.
	MethodRedirect Method = "redirect"
	// MethodCredentials posts username and password as JSON to /auth/{id}/login.
	MethodCredentials Method = "credentials"
)

// Identity is an authenticated admin.
type Identity struct {
	Subject     string
	DisplayName string
	// Email is set only if the provider verified it: provisioned accounts
	// are matched by email.
	Email string
}

// Provider authenticates admins.
type Provider interface {
	Method() Method
	// Available reports whether logins currently work.
	Available() bool
	// Routes registers the provider's handlers below /auth/{id}/. It is
	// called once at startup, and may start background work such as
	// discovering an identity provider.
	Routes(mux *http.ServeMux, core *Core)
}

// Directory is implemented by providers that know all their users, whose
// accounts are then named by subject instead of email.
type Directory interface {
	// Users returns the subjects of all users, sorted. If they cannot be
	// reloaded, it returns the users it knows with the error.
	Users() ([]string, error)
}

// ParseEmail returns a plain email address in lowercase, or false if s is
// none.
func ParseEmail(s string) (string, bool) {
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return "", false
	}
	return strings.ToLower(s), true
}

// Factory creates a provider from its configuration variables, recording
// errors in env.
type Factory func(env *config.Env, cfg config.Config) Provider

var factories = map[string]Factory{}

// Register makes a provider selectable by SITREP_AUTH=id.
func Register(id string, f Factory) {
	factories[id] = f
}

// NewProvider creates the provider selected by cfg.Auth.
func NewProvider(env *config.Env, cfg config.Config) Provider {
	f, ok := factories[cfg.Auth]
	if !ok {
		ids := make([]string, 0, len(factories))
		for id := range factories {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		env.Errorf("SITREP_AUTH", "must be one of %s", strings.Join(ids, ", "))
		return nil
	}
	return f(env, cfg)
}

// ReturnPath returns p if it is a relative path below /admin, else the
// console's home.
func ReturnPath(p string) string {
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" ||
		strings.HasPrefix(p, "//") || strings.Contains(p, `\`) {
		return "/admin/"
	}

	clean := path.Clean(u.Path)
	if clean != "/admin" && !strings.HasPrefix(clean, "/admin/") {
		return "/admin/"
	}
	return p
}

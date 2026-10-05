//go:build testauth

// Package bypass signs in fixed test users without asking for credentials.
// It only exists in builds with the testauth tag and must never be part of
// a release.
package bypass

import (
	"cmp"
	"maps"
	"net/http"
	"slices"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
	"github.com/digineo/sitrep/internal/httpx"
)

func init() {
	auth.Register("bypass", func(*config.Env, config.Config) auth.Provider {
		return provider{}
	})
}

type provider struct{}

// users are the display names of the test users, by subject.
var users = map[string]string{
	"test-admin":      "Test Admin",
	"test-maintainer": "Test Maintainer",
	"test-responder":  "Test Responder",
}

func (provider) Users() ([]string, error) {
	return slices.Sorted(maps.Keys(users)), nil
}

func (provider) Method() auth.Method { return auth.MethodRedirect }

func (provider) Available() bool { return true }

func (provider) Routes(mux *http.ServeMux, core *auth.Core) {
	core.Log.Warn("THE BYPASS AUTH PROVIDER IS ACTIVE: ANYONE CAN SIGN IN AS ADMIN. NEVER USE THIS OUTSIDE OF TESTS.")

	// ?as=<subject> picks the test user, by default test-admin.
	handler := func(w http.ResponseWriter, r *http.Request) {
		subject := cmp.Or(r.URL.Query().Get("as"), "test-admin")
		name, ok := users[subject]
		if !ok {
			http.Error(w, "unknown test user", http.StatusBadRequest)
			return
		}

		id := auth.Identity{
			Subject:     subject,
			DisplayName: name,
		}
		if err := core.Login(w, r, id); err != nil {
			httpx.WriteError(w, r, core.Log, err)
			return
		}
		target := auth.ReturnPath(r.URL.Query().Get("return"))
		http.Redirect(w, r, target, http.StatusSeeOther)
	}

	mux.HandleFunc("GET /auth/bypass/login", handler)
}

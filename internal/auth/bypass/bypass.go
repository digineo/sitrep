//go:build testauth

// Package bypass signs in a fixed test admin without asking for
// credentials. It only exists in builds with the testauth tag and must
// never be part of a release.
package bypass

import (
	"net/http"

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

func (provider) Method() auth.Method { return auth.MethodRedirect }

func (provider) Available() bool { return true }

func (provider) Routes(mux *http.ServeMux, core *auth.Core) {
	core.Log.Warn("THE BYPASS AUTH PROVIDER IS ACTIVE: ANYONE CAN SIGN IN AS ADMIN. NEVER USE THIS OUTSIDE OF TESTS.")

	handler := func(w http.ResponseWriter, r *http.Request) {
		id := auth.Identity{
			Subject:     "test-admin",
			DisplayName: "Test Admin",
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

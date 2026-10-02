// Package oidc signs admins in with OpenID Connect.
package oidc

import (
	"net/http"
	"slices"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/config"
)

func init() {
	auth.Register("oidc", New)
}

type provider struct{}

// New validates the SITREP_OIDC_* variables.
func New(env *config.Env, cfg config.Config) auth.Provider {
	const redirect = "SITREP_OIDC_REDIRECT_URL"
	env.URL("SITREP_OIDC_ISSUER", true)
	env.Required("SITREP_OIDC_CLIENT_ID")
	env.Secret("SITREP_OIDC_CLIENT_SECRET")
	if u := env.URL(redirect, true); u != nil {
		if u.Path != "/auth/oidc/callback" {
			env.Errorf(redirect, "path must be /auth/oidc/callback")
		}

		if !slices.Contains(cfg.BaseDomains, u.Hostname()) {
			env.Errorf(redirect, "host must be one of SITREP_BASE_DOMAINS")
		}
	}

	env.String("SITREP_OIDC_SCOPES", "openid profile email")
	env.String("SITREP_OIDC_GROUPS_CLAIM", "groups")
	env.Required("SITREP_OIDC_ADMIN_GROUP")
	return provider{}
}

func (provider) Method() auth.Method { return auth.MethodRedirect }

// Available is false: this build validates the OIDC configuration, but
// cannot sign in with it yet.
func (provider) Available() bool { return false }

func (provider) Routes(*http.ServeMux, *auth.Core) {}

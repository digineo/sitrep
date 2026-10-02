package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/config"
)

func validate(vars map[string]string) error {
	env := config.NewEnv(func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	})
	New(env, config.Config{BaseDomains: []string{"status.example.com"}})
	return env.Err()
}

func TestConfig(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	valid := map[string]string{
		"SITREP_OIDC_ISSUER":       "https://idp.example.com/realms/acme",
		"SITREP_OIDC_CLIENT_ID":    "sitrep",
		"SITREP_OIDC_REDIRECT_URL": "https://status.example.com/auth/oidc/callback",
		"SITREP_OIDC_ADMIN_GROUP":  "admins",
	}
	require.NoError(validate(valid))

	err := validate(map[string]string{})
	required := []string{"ISSUER", "CLIENT_ID", "REDIRECT_URL", "ADMIN_GROUP"}
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

		assert.ErrorContains(validate(vars), name, value)
	}

	valid["SITREP_OIDC_REDIRECT_URL"] = "https://other.example.com/auth/oidc/callback"
	err = validate(valid)
	assert.ErrorContains(err, "host must be one of SITREP_BASE_DOMAINS")
}

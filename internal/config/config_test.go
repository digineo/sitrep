package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envOf(vars map[string]string) *Env {
	return NewEnv(func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	})
}

func load(t *testing.T, vars map[string]string) (Config, error) {
	t.Helper()
	if _, ok := vars["SITREP_BASE_DOMAINS"]; !ok {
		vars["SITREP_BASE_DOMAINS"] = "status.example.com"
	}
	env := envOf(vars)
	return Load(env), env.Err()
}

func TestDefaults(t *testing.T) {
	c, err := load(t, map[string]string{})
	require.NoError(t, err)
	want := Config{
		Listen:         ":2607",
		DB:             "sitrep.db",
		DefaultRefresh: 30 * time.Second,
		BaseDomains:    []string{"status.example.com"},
		Auth:           "oidc",
		SessionTTL:     12 * time.Hour,
		LogLevel:       slog.LevelInfo,
		LogFormat:      "text",
	}
	assert.Equal(t, want, c)
}

func TestStrings(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	c, err := load(t, map[string]string{
		"SITREP_LISTEN": "127.0.0.1:8080",
		"SITREP_DB":     "/var/lib/sitrep.db",
	})
	require.NoError(err)
	assert.Equal("127.0.0.1:8080", c.Listen)
	assert.Equal("/var/lib/sitrep.db", c.DB)

	_, err = load(t, map[string]string{"SITREP_LISTEN": ""})
	assert.ErrorContains(err, "SITREP_LISTEN: must not be empty")
}

func TestBooleans(t *testing.T) {
	for in, want := range map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   true,
		"On":    true,
		"false": false,
		"0":     false,
		"no":    false,
		"OFF":   false,
	} {
		c, err := load(t, map[string]string{"SITREP_TRUST_PROXY": in})
		require.NoError(t, err, in)
		assert.Equal(t, want, c.TrustProxy, in)
	}

	for _, in := range []string{"", "y", "2", "enabled"} {
		_, err := load(t, map[string]string{"SITREP_TRUST_PROXY": in})
		assert.ErrorContains(t, err, "SITREP_TRUST_PROXY", in)
	}
}

func TestDurations(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	c, err := load(t, map[string]string{
		"SITREP_DEFAULT_REFRESH": "1m30s",
		"SITREP_SESSION_TTL":     "30d",
	})
	require.NoError(err)
	assert.Equal(90*time.Second, c.DefaultRefresh)
	assert.Equal(30*24*time.Hour, c.SessionTTL)

	for name, values := range map[string][]string{
		"SITREP_DEFAULT_REFRESH": {"4s", "1d1s", "30", "", "1.5m"},
		"SITREP_SESSION_TTL":     {"4m", "31d", "12 h"},
	} {
		for _, v := range values {
			_, err := load(t, map[string]string{name: v})
			assert.ErrorContains(err, name, v)
		}
	}
}

func TestEnums(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	c, err := load(t, map[string]string{
		"SITREP_LOG_LEVEL":  "debug",
		"SITREP_LOG_FORMAT": "pretty",
		"SITREP_AUTH":       "basic",
	})
	require.NoError(err)
	assert.Equal(slog.LevelDebug, c.LogLevel)
	assert.Equal("pretty", c.LogFormat)
	assert.Equal("basic", c.Auth)

	for name, v := range map[string]string{
		"SITREP_LOG_LEVEL":  "warning",
		"SITREP_LOG_FORMAT": "JSON",
	} {
		_, err := load(t, map[string]string{name: v})
		assert.ErrorContains(err, name)
	}
}

func TestSecretKey(t *testing.T) {
	key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 bytes
	c, err := load(t, map[string]string{"SITREP_SECRET_KEY": key})
	require.NoError(t, err)
	assert.Equal(t, []byte("0123456789abcdef0123456789abcdef"), c.SecretKey)

	for _, v := range []string{"MDEyMzQ1Njc4OWFiY2RlZg==", "not base64!"} {
		_, err := load(t, map[string]string{"SITREP_SECRET_KEY": v})
		assert.ErrorContains(t, err, "SITREP_SECRET_KEY", v)
	}
}

func TestBaseDomains(t *testing.T) {
	c, err := load(t, map[string]string{
		"SITREP_BASE_DOMAINS": "status.example.com, sitrep.localhost",
	})
	require.NoError(t, err)
	want := []string{"status.example.com", "sitrep.localhost"}
	assert.Equal(t, want, c.BaseDomains)

	for _, v := range []string{
		"",
		"localhost",
		"Example.com",
		"a.example.com,a.example.com",
		"a.example.com,,b.example.com",
		"a.example.com:80",
	} {
		_, err := load(t, map[string]string{"SITREP_BASE_DOMAINS": v})
		assert.ErrorContains(t, err, "SITREP_BASE_DOMAINS", v)
	}
}

func TestErrorsAreCollected(t *testing.T) {
	_, err := load(t, map[string]string{
		"SITREP_BASE_DOMAINS": "",
		"SITREP_TRUST_PROXY":  "maybe",
		"SITREP_LOG_LEVEL":    "loud",
	})
	require.Error(t, err)
	for _, name := range []string{
		"SITREP_BASE_DOMAINS",
		"SITREP_TRUST_PROXY",
		"SITREP_LOG_LEVEL",
	} {
		assert.Contains(t, err.Error(), name)
	}
}

func TestLogAttrsRedactSecrets(t *testing.T) {
	assert := assert.New(t)

	env := envOf(map[string]string{
		"SITREP_BASE_DOMAINS": "status.example.com",
		"SITREP_SECRET_KEY":   "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		"SECRET":              "hunter2",
		"ISSUER":              "https://user:pass@idp.example.com/realm",
	})
	Load(env)
	env.Secret("SECRET")
	env.URL("ISSUER", true)

	var b strings.Builder
	for _, a := range env.LogAttrs() {
		b.WriteString(a.String() + " ")
	}

	logged := b.String()
	assert.NotContains(logged, "MDEyMzQ1")
	assert.NotContains(logged, "hunter2")
	assert.NotContains(logged, "pass@")
	assert.Contains(logged, "SITREP_BASE_DOMAINS=status.example.com")
}

func TestDotenv(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	vars, err := parseDotenv(`
# comment
A=1
export B = two words
C="quoted # not a comment"
D='single'
E=value # comment
F=
`)
	require.NoError(err)
	want := map[string]string{
		"A": "1",
		"B": "two words",
		"C": "quoted # not a comment",
		"D": "single",
		"E": "value",
		"F": "",
	}
	assert.Equal(want, vars)

	for _, data := range []string{"novalue", "1A=x", "A=\"open", "A='x\""} {
		_, err := parseDotenv("X=1\n" + data)
		assert.ErrorContains(err, "line 2", data)
	}

	_, err = parseDotenv("A=\"secret")
	assert.NotContains(err.Error(), "secret", "errors never contain the line")
}

func TestReadDotenvPrecedence(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir := t.TempDir()
	local := filepath.Join(dir, ".env.local")
	plain := filepath.Join(dir, ".env")
	require.NoError(os.WriteFile(local, []byte("SITREP_TEST_A=local\n"), 0o600))
	data := "SITREP_TEST_A=plain\nSITREP_TEST_B=plain\nSITREP_TEST_C=plain\n"
	require.NoError(os.WriteFile(plain, []byte(data), 0o600))

	vars, err := ReadDotenv(local, plain, filepath.Join(dir, "missing"))
	require.NoError(err)
	t.Setenv("SITREP_TEST_C", "real")
	lookup := Lookup(vars)
	for name, want := range map[string]string{
		"SITREP_TEST_A": "local",
		"SITREP_TEST_B": "plain",
		"SITREP_TEST_C": "real",
	} {
		v, ok := lookup(name)
		assert.True(ok)
		assert.Equal(want, v, name)
	}

	require.NoError(os.WriteFile(plain, []byte("broken"), 0o600))
	_, err = ReadDotenv(local, plain)
	assert.ErrorContains(err, ".env: line 1")
}

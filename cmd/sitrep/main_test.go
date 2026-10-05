package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth/basic"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

func runCLI(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestUsage(t *testing.T) {
	assert := assert.New(t)

	for _, arg := range []string{"help", "-h", "--help"} {
		code, stdout, stderr := runCLI("", arg)
		assert.Equal(0, code, arg)
		assert.Contains(stdout, "Usage: sitrep", arg)
		assert.Empty(stderr, arg)
	}

	for _, args := range [][]string{nil, {"nope"}} {
		code, stdout, stderr := runCLI("", args...)
		assert.Equal(2, code, args)
		assert.Empty(stdout, args)
		assert.Contains(stderr, "Usage: sitrep", args)
	}

	code, stdout, _ := runCLI("", "version")
	assert.Equal(0, code)
	assert.Equal("version untagged\ncommit  unknown\nbuilt   unknown\n", stdout)

	code, _, _ = runCLI("", "version", "extra")
	assert.Equal(2, code)

	code, _, _ = runCLI("", "serve", "extra")
	assert.Equal(2, code)

	code, _, _ = runCLI("", "serve", "-flag")
	assert.Equal(2, code)
}

func TestHashPassword(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	code, stdout, stderr := runCLI("correct horse\r\nignored\n", "hash-password")
	require.Equal(0, code, stderr)
	hash := strings.TrimSuffix(stdout, "\n")
	assert.True(strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$"))
	assert.True(
		basic.Verify(hash, "correct horse"),
		"the first line without its ending is the password",
	)

	code, stdout, _ = runCLI("correct horse", "hash-password", "-user", "ann")
	require.Equal(0, code)
	user, hash, ok := strings.Cut(strings.TrimSuffix(stdout, "\n"), ":")
	require.True(ok)
	assert.Equal("ann", user)
	assert.True(basic.Verify(hash, "correct horse"))

	for _, stdin := range []string{"", "\n"} {
		code, stdout, stderr := runCLI(stdin, "hash-password")
		assert.Equal(1, code)
		assert.Empty(stdout)
		assert.Contains(stderr, "must not be empty")
	}

	for _, name := range []string{"a:b", "a b", "a\tb"} {
		code, stdout, stderr := runCLI("pw", "hash-password", "-user", name)
		assert.Equal(2, code, name)
		assert.Empty(stdout, name)
		assert.Contains(stderr, "colons or whitespace", name)
	}

	code, _, _ = runCLI("pw", "hash-password", "extra")
	assert.Equal(2, code)
}

func TestPromptPassword(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	entries := func(values ...string) func() ([]byte, error) {
		return func() ([]byte, error) {
			v := values[0]
			values = values[1:]
			return []byte(v), nil
		}
	}

	var stderr bytes.Buffer
	pw, err := promptPassword(entries("same", "same"), &stderr)
	require.NoError(err)
	assert.Equal("same", pw)
	assert.Contains(stderr.String(), "Repeat password")

	_, err = promptPassword(entries("one", "two"), &stderr)
	assert.ErrorContains(err, "do not match")

	closed := func() ([]byte, error) { return nil, errors.New("closed") }
	_, err = promptPassword(closed, &stderr)
	assert.ErrorContains(err, "closed")
}

// The bypass provider signs in anyone. Only testauth builds may contain it.
func TestReleaseBuildExcludesBypass(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	deps := func(args ...string) string {
		argv := append([]string{"list", "-deps"}, append(args, ".")...)
		out, err := exec.Command("go", argv...).Output()
		require.NoError(err)
		return string(out)
	}

	const bypass = "github.com/digineo/sitrep/internal/auth/bypass\n"
	assert.NotContains(deps(), bypass)
	assert.NotContains(deps("-tags", "dev"), bypass)
	assert.Contains(deps("-tags", "testauth"), bypass)
}

func TestServeConfigErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("SITREP_BASE_DOMAINS", "localhost")
	t.Setenv("SITREP_AUTH", "basic")
	t.Setenv("SITREP_LOG_LEVEL", "loud")
	code, stdout, stderr := runCLI("", "serve")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	for _, name := range []string{
		"SITREP_BASE_DOMAINS",
		"SITREP_BASIC_USERS_FILE",
		"SITREP_LOG_LEVEL",
	} {
		assert.Contains(t, stderr, name)
	}
}

func TestGrantOwner(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	t.Chdir(t.TempDir())
	t.Setenv("SITREP_BASE_DOMAINS", "status.example.com")
	t.Setenv("SITREP_AUTH", "basic")
	t.Setenv("SITREP_BASIC_USERS_FILE", "users")
	require.NoError(os.WriteFile("users", []byte("ann:"+basic.Hash("pw")+"\n"), 0o600))

	code, _, _ := runCLI("", "grant-owner")
	assert.Equal(2, code)

	code, stdout, stderr := runCLI("", "grant-owner", "ann")
	require.Equal(0, code, stderr)
	assert.Equal("ann is an owner\n", stdout)
	assert.Contains(stderr, "made an account an owner", "logged")

	code, _, stderr = runCLI("", "grant-owner", "bob")
	assert.Equal(1, code)
	assert.Contains(stderr, `the basic provider has no user or account "bob"`)

	db, err := store.Open("sitrep.db")
	require.NoError(err)
	acc, _, _, err := db.SignIn("basic", "ann", "Ann", "")
	require.NoError(err)
	assert.Equal(model.RoleOwner, acc.Role)

	code, _, stderr = runCLI("", "grant-owner", "ann")
	assert.Equal(1, code, "the server holds the database")
	assert.Contains(stderr, "in use by another process")
	require.NoError(db.Close())

	t.Setenv("SITREP_AUTH", "oidc")
	t.Setenv("SITREP_OIDC_ISSUER", "https://idp.example.com/realms/acme")
	t.Setenv("SITREP_OIDC_CLIENT_ID", "sitrep")
	t.Setenv("SITREP_OIDC_REDIRECT_URL", "https://status.example.com/auth/oidc/callback")
	t.Setenv("SITREP_OIDC_GROUP", "admins")
	code, stdout, stderr = runCLI("", "grant-owner", "Carl@Example.com")
	require.Equal(0, code, stderr)
	assert.Contains(stdout, "carl@example.com is an owner from their next sign-in")

	code, _, stderr = runCLI("", "grant-owner", "Carl <carl@example.com>")
	assert.Equal(1, code)
	assert.Contains(stderr, "is neither an account ID nor an email address")

	// Dora's identity provider verifies no emails: she is named by her
	// account's ID.
	db, err = store.Open("sitrep.db")
	require.NoError(err)
	acc, _, _, err = db.SignIn("oidc", "u-1", "Carl", "carl@example.com")
	require.NoError(err)
	assert.Equal(model.RoleOwner, acc.Role)
	dora, _, _, err := db.SignIn("oidc", "u-2", "Dora", "")
	require.NoError(err)
	require.Equal(model.RoleNone, dora.Role)
	require.NoError(db.Close())

	code, stdout, stderr = runCLI("", "grant-owner", dora.ID)
	require.Equal(0, code, stderr)
	assert.Equal(dora.ID+" is an owner\n", stdout)

	db, err = store.Open("sitrep.db")
	require.NoError(err)
	defer func() { _ = db.Close() }()
	acc, _, _, err = db.SignIn("oidc", "u-2", "Dora", "")
	require.NoError(err)
	assert.Equal(model.RoleOwner, acc.Role)
}

func TestHealthURL(t *testing.T) {
	assert := assert.New(t)

	for listen, want := range map[string]string{
		":2607":          "http://127.0.0.1:2607/healthz",
		"0.0.0.0:2607":   "http://127.0.0.1:2607/healthz",
		"[::]:2607":      "http://127.0.0.1:2607/healthz",
		"10.0.0.1:80":    "http://10.0.0.1:80/healthz",
		"[::1]:2607":     "http://[::1]:2607/healthz",
		"localhost:2607": "http://localhost:2607/healthz",
	} {
		got, err := healthURL(listen)
		assert.NoError(err, listen)
		assert.Equal(want, got, listen)
	}

	_, err := healthURL("2607")
	assert.Error(err)
}

func TestHealthcheck(t *testing.T) {
	assert := assert.New(t)
	t.Chdir(t.TempDir())

	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal("/healthz", r.URL.Path)
		w.WriteHeader(status)
	}))
	t.Setenv("SITREP_LISTEN", srv.Listener.Addr().String())

	code, stdout, stderr := runCLI("", "healthcheck")
	assert.Equal(0, code, stderr)
	assert.Empty(stdout)

	status = http.StatusServiceUnavailable
	code, _, stderr = runCLI("", "healthcheck")
	assert.Equal(1, code)
	assert.Contains(stderr, "503")

	srv.Close()
	code, _, _ = runCLI("", "healthcheck")
	assert.Equal(1, code)

	code, _, _ = runCLI("", "healthcheck", "extra")
	assert.Equal(2, code)
}

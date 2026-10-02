package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth/basic"
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

	code, _, _ := runCLI("", "serve", "extra")
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

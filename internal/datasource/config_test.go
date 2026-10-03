package datasource

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// testType is configured like a typical backend: a URL, an auth mode with
// dependent credentials, and a timeout.
type testType struct{}

func (testType) Fields() []Field {
	return []Field{
		{
			Name:     "url",
			Kind:     KindURL,
			Required: true,
		},
		{
			Name:    "auth",
			Kind:    KindSelect,
			Default: "none",
			Options: []string{"none", "basic", "bearer"},
		},
		{
			Name:     "username",
			Kind:     KindText,
			Required: true,
			When:     &Condition{"auth", "basic"},
		},
		{
			Name: "password",
			Kind: KindSecret,
			When: &Condition{"auth", "basic"},
		},
		{
			Name:     "token",
			Kind:     KindSecret,
			Required: true,
			When:     &Condition{"auth", "bearer"},
		},
		{
			Name:    "timeout",
			Kind:    KindDuration,
			Default: "10s",
			Min:     "1s",
			Max:     "2m",
		},
		{
			Name: "verbose",
			Kind: KindBool,
		},
	}
}
func (testType) PanelTypes() []string { return []string{model.PanelStat} }
func (testType) Evaluate(
	context.Context,
	Config,
	model.Panel,
	time.Time,
) (Result, error) {
	return Result{}, nil
}
func (testType) Test(context.Context, Config) (string, error) { return "", nil }
func (testType) Summary(cfg Config) string                    { return cfg["url"] }

func init() {
	Register("test", testType{})
}

var key = bytes.Repeat([]byte{7}, 32)

// apply runs Apply and returns its field errors by path.
func apply(
	t *testing.T,
	ds *model.DataSource,
	in map[string]string,
	key []byte,
) map[string]string {
	t.Helper()
	var f apierr.Fields
	require.NoError(t, Apply(&f, ds, in, key))
	if len(f) == 0 {
		return nil
	}

	out := map[string]string{}
	for _, e := range f {
		out[e.Path] = e.Code
	}
	return out
}

func TestApply(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	var f apierr.Fields
	assert.Equal("Main", ValidateName(&f, "  Main "))
	assert.Empty(f)
	ValidateName(&f, " ")
	ValidateName(&f, strings.Repeat("ä", 101))
	errs := apierr.Fields{
		{
			Path: "name",
			Code: "required",
		},
		{
			Path: "name",
			Code: "too_long",
		},
	}
	assert.Equal(errs, f)

	ds := &model.DataSource{
		ID:   "a",
		Type: "test",
	}
	in := map[string]string{
		"url":     "https://prom.example.com/prefix/",
		"timeout": "90s",
		"verbose": "true",
	}
	require.Nil(apply(t, ds, in, nil))
	want := map[string]string{
		"url":     "https://prom.example.com/prefix",
		"auth":    "none",
		"timeout": "1m30s",
		"verbose": "true",
	}
	assert.Equal(want, ds.Config)

	in = map[string]string{
		"url":      "http://prom",
		"username": "ann",
	}
	require.Nil(apply(t, ds, in, nil))
	assert.NotContains(
		ds.Config,
		"username",
		"fields hidden by their condition are dropped",
	)

	want = map[string]string{
		"config.url":      "required",
		"config.username": "required",
	}
	assert.Equal(want, apply(t, ds, map[string]string{"auth": "basic"}, nil))

	in = map[string]string{
		"url":  "http://prom",
		"auth": "bearer",
	}
	want = map[string]string{"config.token": "required"}
	assert.Equal(want, apply(t, ds, in, nil))

	for in, want := range map[[2]string]string{
		{"url", "ftp://prom"}:                                "invalid_url",
		{"url", "https://user:pw@prom"}:                      "invalid_url",
		{"url", "https://prom/?q=1"}:                         "invalid_url",
		{"url", "https://prom/#f"}:                           "invalid_url",
		{"url", "prom:9090"}:                                 "invalid_url",
		{"auth", "digest"}:                                   "invalid_value",
		{"timeout", "500ms"}:                                 "invalid_duration",
		{"timeout", "3m"}:                                    "out_of_range",
		{"timeout", "0s"}:                                    "out_of_range",
		{"verbose", "yes"}:                                   "invalid_value",
		{"unknown", "x"}:                                     "invalid_value",
		{"url", "https://prom/" + strings.Repeat("a", 2000)}: "too_long",
	} {
		cfg := map[string]string{"url": "http://prom"}
		cfg[in[0]] = in[1]
		fresh := &model.DataSource{
			ID:   "a",
			Type: "test",
		}
		got := apply(t, fresh, cfg, nil)
		assert.Equal(map[string]string{"config." + in[0]: want}, got, in)
	}
}

func TestSecrets(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	ds := &model.DataSource{
		ID:   "a",
		Type: "test",
	}
	cfg := map[string]string{
		"url":      "http://prom",
		"auth":     "basic",
		"username": "ann",
		"password": "hunter2",
	}
	want := map[string]string{"config.password": "secret_key_missing"}
	assert.Equal(want, apply(t, ds, cfg, nil))

	require.Nil(apply(t, ds, cfg, key))
	assert.NotContains(ds.Config, "password")
	require.Contains(ds.Secrets, "password")
	assert.False(bytes.Contains(ds.Secrets["password"], []byte("hunter2")))
	opened, err := Open(*ds, key)
	require.NoError(err)
	assert.Equal("hunter2", opened["password"])

	delete(cfg, "password")
	require.Nil(
		apply(t, ds, cfg, nil),
		"an omitted secret keeps the stored value, even without key",
	)
	opened, err = Open(*ds, key)
	require.NoError(err)
	assert.Equal("hunter2", opened["password"])

	cfg["password"] = "correct horse"
	require.Nil(apply(t, ds, cfg, key))
	opened, err = Open(*ds, key)
	require.NoError(err)
	assert.Equal(
		"correct horse",
		opened["password"],
		"a new value replaces the secret",
	)

	moved := *ds
	moved.ID = "b"
	_, err = Open(moved, key)
	assert.ErrorIs(err, ErrUnusable, "sealed values are bound to their data source")
	_, err = Open(*ds, bytes.Repeat([]byte{8}, 32))
	assert.ErrorIs(err, ErrUnusable, "another key")
	_, err = Open(*ds, nil)
	assert.ErrorIs(err, ErrUnusable, "no key")

	cfg["password"] = ""
	require.Nil(apply(t, ds, cfg, nil))
	assert.Empty(ds.Secrets, "an empty value clears the secret")

	cfg = map[string]string{
		"url":   "http://prom",
		"auth":  "bearer",
		"token": "t0ken",
	}
	require.Nil(apply(t, ds, cfg, key))
	delete(cfg, "token")
	cfg["url"] = "http://prom/"
	require.Nil(
		apply(t, ds, cfg, nil),
		"the same URL after normalization keeps the secret",
	)
	require.Contains(ds.Secrets, "token")
	cfg["url"] = "http://other"
	want = map[string]string{"config.token": "required"}
	assert.Equal(
		want,
		apply(t, ds, cfg, nil),
		"another URL discards the stored secret, "+
			"so a required one must be entered again",
	)
	require.Contains(ds.Secrets, "token", "a failed validation changes nothing")
	cfg["token"] = "n3w"
	require.Nil(
		apply(t, ds, cfg, key),
		"secrets submitted with the new URL are stored",
	)
	opened, err = Open(*ds, key)
	require.NoError(err)
	assert.Equal("n3w", opened["token"])

	cfg = map[string]string{
		"url":      "http://prom",
		"auth":     "basic",
		"username": "ann",
		"password": "hunter2",
	}
	require.Nil(apply(t, ds, cfg, key))
	delete(cfg, "password")
	cfg["url"] = "http://other"
	require.Nil(apply(t, ds, cfg, nil))
	assert.Empty(ds.Secrets, "an optional secret is discarded with the URL change")

	cfg["password"] = "hunter2"
	require.Nil(apply(t, ds, cfg, key))
	cfg["auth"] = "none"
	delete(cfg, "password")
	require.Nil(apply(t, ds, cfg, key))
	assert.Empty(ds.Secrets, "hidden secrets are dropped")
}

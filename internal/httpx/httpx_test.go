package httpx

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

func TestEffective(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		tls     bool
		headers map[string]string
		trust   bool
		want    Info
	}{
		{"plain", "Status.Example.com:8080", false, nil, false,
			Info{Host: "status.example.com", Origin: "http://status.example.com:8080", Scheme: "http", IP: "192.0.2.1"}},
		{"default port and trailing dot", "example.com.:80", false, nil, false,
			Info{Host: "example.com", Origin: "http://example.com", Scheme: "http", IP: "192.0.2.1"}},
		{"tls", "example.com:443", true, nil, false,
			Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "192.0.2.1"}},
		{"ipv6", "[::1]:2607", false, nil, false,
			Info{Host: "::1", Origin: "http://[::1]:2607", Scheme: "http", IP: "192.0.2.1"}},
		{"proxy headers ignored without trust", "internal:2607", false,
			map[string]string{"X-Forwarded-Host": "example.com", "X-Forwarded-Proto": "https", "X-Forwarded-For": "198.51.100.7"}, false,
			Info{Host: "internal", Origin: "http://internal:2607", Scheme: "http", IP: "192.0.2.1"}},
		{"trusted proxy", "internal:2607", false,
			map[string]string{"X-Forwarded-Host": "Example.com, other.example", "X-Forwarded-Proto": "https", "X-Forwarded-For": "203.0.113.9, 198.51.100.7"}, true,
			Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "198.51.100.7"}},
		{"invalid forwarded values fall back", "internal", false,
			map[string]string{"X-Forwarded-Host": "bad host!", "X-Forwarded-Proto": "HTTPS", "X-Forwarded-For": "unknown"}, true,
			Info{Host: "internal", Origin: "http://internal", Scheme: "http", IP: "192.0.2.1"}},
		{"forwarded host with invalid port", "internal", false,
			map[string]string{"X-Forwarded-Host": "example.com:99999"}, true,
			Info{Host: "internal", Origin: "http://internal", Scheme: "http", IP: "192.0.2.1"}},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = tt.host
		r.RemoteAddr = "192.0.2.1:4711"
		if tt.tls {
			r.TLS = &tls.ConnectionState{}
		}

		for k, v := range tt.headers {
			r.Header.Set(k, v)
		}

		assert.Equal(t, tt.want, Effective(r, tt.trust), tt.name)
	}
}

func TestReadJSON(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	type body struct {
		A string `json:"a"`
	}
	read := func(data string) (body, error) {
		var b body
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(data))
		err := ReadJSON(httptest.NewRecorder(), r, &b)
		return b, err
	}

	b, err := read(`{"a": "x"}`)
	require.NoError(err)
	assert.Equal("x", b.A)

	for data, code := range map[string]string{
		`{"a": "x", "b": 1}`: apierr.InvalidJSON,
		`{"a": "x"} {}`:      apierr.InvalidJSON,
		`{"a": `:             apierr.InvalidJSON,
		`{"a": "` + strings.Repeat("x", MaxBody) + `"}`: apierr.TooLarge,
	} {
		_, err := read(data)
		e, ok := errors.AsType[*apierr.Error](err)
		require.True(ok)
		assert.Equal(code, e.Code)
	}
}

func TestWriteError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	WriteError(w, r, xlog.NewDiscard(), errors.New("secret internal detail"))
	assert.Equal(http.StatusInternalServerError, w.Code)
	assert.NotContains(w.Body.String(), "secret")
	assert.Empty(w.Header().Get("Cache-Control"))

	w = httptest.NewRecorder()
	var f apierr.Fields
	f.Add("name", apierr.Required)
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	WriteError(w, r, xlog.NewDiscard(), f.Err())
	assert.Equal(http.StatusBadRequest, w.Code)
	var res struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Fields  []apierr.Field `json:"fields"`
		} `json:"error"`
	}
	require.NoError(json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(apierr.Invalid, res.Error.Code)
	assert.NotEmpty(res.Error.Message)
	want := []apierr.Field{{
		Path: "name",
		Code: "required",
	}}
	assert.Equal(want, res.Error.Fields)
}

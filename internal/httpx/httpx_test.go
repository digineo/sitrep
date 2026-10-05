package httpx

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

func TestEffective(t *testing.T) {
	proxies := func(prefixes ...string) Proxies {
		var p Proxies
		for _, s := range prefixes {
			p.Prefixes = append(p.Prefixes, netip.MustParsePrefix(s))
		}
		return p
	}
	none := Proxies{}
	all := Proxies{All: true}
	forwarded := map[string]string{
		"X-Forwarded-Host":  "Example.com, other.example",
		"X-Forwarded-Proto": "https",
		"X-Forwarded-For":   "203.0.113.9, 198.51.100.7",
	}

	tests := []struct {
		name    string
		host    string
		tls     bool
		headers map[string]string
		trust   Proxies
		want    Info
	}{
		{
			name:    "plain",
			host:    "Status.Example.com:8080",
			tls:     false,
			headers: nil,
			trust:   none,
			want:    Info{Host: "status.example.com", Origin: "http://status.example.com:8080", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "default port and trailing dot",
			host:    "example.com.:80",
			tls:     false,
			headers: nil,
			trust:   none,
			want:    Info{Host: "example.com", Origin: "http://example.com", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "tls",
			host:    "example.com:443",
			tls:     true,
			headers: nil,
			trust:   none,
			want:    Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "192.0.2.1"},
		}, {
			name:    "ipv6",
			host:    "[::1]:2607",
			tls:     false,
			headers: nil,
			trust:   none,
			want:    Info{Host: "::1", Origin: "http://[::1]:2607", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "proxy headers ignored without trust",
			host:    "internal:2607",
			tls:     false,
			headers: forwarded,
			trust:   none,
			want:    Info{Host: "internal", Origin: "http://internal:2607", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "proxy headers ignored from untrusted peers",
			host:    "internal:2607",
			tls:     false,
			headers: forwarded,
			trust:   proxies("10.0.0.0/8", "192.0.2.2/32"),
			want:    Info{Host: "internal", Origin: "http://internal:2607", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "trusted proxy",
			host:    "internal:2607",
			tls:     false,
			headers: forwarded,
			trust:   proxies("192.0.2.1/32"),
			want:    Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "198.51.100.7"},
		}, {
			name:    "trusted proxies are skipped",
			host:    "internal:2607",
			tls:     false,
			headers: forwarded,
			trust:   proxies("192.0.2.0/24", "198.51.100.0/24"),
			want:    Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "203.0.113.9"},
		}, {
			name:    "every peer trusted, with the last entry",
			host:    "internal:2607",
			tls:     false,
			headers: forwarded,
			trust:   all,
			want:    Info{Host: "example.com", Origin: "https://example.com", Scheme: "https", IP: "198.51.100.7"},
		}, {
			name:    "invalid forwarded values fall back",
			host:    "internal",
			tls:     false,
			headers: map[string]string{"X-Forwarded-Host": "bad host!", "X-Forwarded-Proto": "HTTPS", "X-Forwarded-For": "unknown"},
			trust:   all,
			want:    Info{Host: "internal", Origin: "http://internal", Scheme: "http", IP: "192.0.2.1"},
		}, {
			name:    "forwarded host with invalid port",
			host:    "internal",
			tls:     false,
			headers: map[string]string{"X-Forwarded-Host": "example.com:99999"},
			trust:   all,
			want:    Info{Host: "internal", Origin: "http://internal", Scheme: "http", IP: "192.0.2.1"},
		},
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

func TestAccessLog(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	var out strings.Builder
	log, err := xlog.New(xlog.AsText(), xlog.WriteTo(&out))
	require.NoError(err)

	status := http.StatusOK
	h := AccessLog(log, Proxies{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	get := func(path string) string {
		out.Reset()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		return out.String()
	}

	assert.Contains(get("/"), "level=INFO")
	assert.Empty(get("/healthz"), "successful health checks log at debug level")

	status = http.StatusServiceUnavailable
	assert.Contains(get("/healthz"), "level=INFO")
}

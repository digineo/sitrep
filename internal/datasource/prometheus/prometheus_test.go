package prometheus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
)

// stub serves handler below a path prefix and returns a configuration for
// it.
func stub(t *testing.T, handler http.HandlerFunc) datasource.Config {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/prefix/", http.StripPrefix("/prefix", handler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return datasource.Config{
		"url":     srv.URL + "/prefix",
		"auth":    "none",
		"timeout": "10s",
	}
}

func reply(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

var now = time.Date(2026, 10, 2, 12, 0, 7, 500e6, time.UTC)

func TestEvaluateInstant(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	var form url.Values
	body := `{"status":"success","data":{"resultType":"vector","result":[
		{"metric":{"job":"a"},"value":[1790942407.5,"1.5"]},
		{"metric":{"job":"b"},"value":[1790942407.5,"NaN"]}]}}`
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal("/api/v1/query", r.URL.Path)
		assert.Equal(http.MethodPost, r.Method)
		require.NoError(r.ParseForm())
		form = r.PostForm
		reply(body)(w, r)
	})

	p := model.Panel{
		Type:  model.PanelStat,
		Query: "up",
	}
	r, err := Type{}.Evaluate(context.Background(), cfg, p, now)
	require.NoError(err)
	want := url.Values{
		"query": {"up"},
		"time":  {"1790942407.5"},
	}
	assert.Equal(want, form)
	assert.Equal(len(body), r.Bytes, "the size of the response")
	require.Len(r.Samples, 2)
	sample := datasource.Sample{
		Labels: map[string]string{"job": "a"},
		Value:  1.5,
	}
	assert.Equal(sample, r.Samples[0])
	assert.True(math.IsNaN(r.Samples[1].Value))

	cfg = stub(t, reply(`{"status":"success","data":{"resultType":"scalar","result":[1790942407.5,"+Inf"]}}`))
	p = model.Panel{
		Type:  model.PanelStatus,
		Query: "1",
	}
	r, err = Type{}.Evaluate(context.Background(), cfg, p, now)
	require.NoError(err)
	require.NotNil(r.Scalar)
	assert.True(math.IsInf(*r.Scalar, 1))

	for _, typ := range []string{"matrix", "string"} {
		cfg = stub(t, reply(`{"status":"success","data":{"resultType":"`+typ+`","result":[]}}`))
		p = model.Panel{
			Type:  model.PanelStat,
			Query: "x",
		}
		_, err = Type{}.Evaluate(context.Background(), cfg, p, now)
		assert.ErrorIs(err, errInstant, typ)
	}
}

func TestEvaluateRange(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	var form url.Values
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal("/api/v1/query_range", r.URL.Path)
		require.NoError(r.ParseForm())
		form = r.PostForm
		// 1790942400 is the window's end; points off the grid and outside
		// the window are ignored.
		reply(`{"status":"success","data":{"resultType":"matrix","result":[
			{"metric":{"job":"a"},"values":[[1790938800,"1"],[1790942385,"2"],[1790942400,"3"],[1790942415,"4"]]},
			{"metric":{"job":"b"},"values":[[1790942370,"5"],[1790942371,"6"]]}]}}`)(w, r)
	})

	p := model.Panel{
		Type:  model.PanelTimeseries,
		Query: "up",
		Range: "1h",
	}
	r, err := Type{}.Evaluate(context.Background(), cfg, p, now)
	require.NoError(err)
	want := url.Values{
		"query": {"up"},
		"start": {"1790938800"},
		"end":   {"1790942400"},
		"step":  {"15"},
	}
	assert.Equal(want, form, "1h/240 = 15s, the end aligned to the step")
	assert.Positive(r.Bytes)
	require.Len(r.Times, 241)
	assert.Equal(time.Unix(1790938800, 0).UTC(), r.Times[0])
	assert.Equal(time.Unix(1790942400, 0).UTC(), r.Times[240])
	require.Len(r.Series, 2)
	a, b := r.Series[0].Values, r.Series[1].Values
	assert.Equal([]float64{1, 2, 3}, []float64{a[0], a[239], a[240]})
	assert.True(math.IsNaN(a[1]))
	assert.Equal(5.0, b[238])
	assert.True(math.IsNaN(b[239]))

	cfg = stub(t, reply(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	_, err = Type{}.Evaluate(context.Background(), cfg, p, now)
	assert.ErrorIs(err, errRange)
}

func TestWindow(t *testing.T) {
	tests := []struct {
		rng, step         string
		start, end, steps int64
	}{
		{"1h", "", 1790938800, 1790942400, 15},
		{"1m", "", 1790942347, 1790942407, 1},
		{"2m", "", 1790942287, 1790942407, 1}, // 0.5s rounds up to 1s
		{"7d", "", 1790336520, 1790941320, 2520},
		{"1h", "7s", 1790938805, 1790942405, 7},
		{"1d", "1h", 1790856000, 1790942400, 3600},
	}
	for _, tt := range tests {
		p := model.Panel{
			Range: tt.rng,
			Step:  tt.step,
		}
		start, end, step := window(p, now)
		want := [3]int64{tt.start, tt.end, tt.steps}
		got := [3]int64{start, end, step}
		assert.Equal(t, want, got, "%s/%s", tt.rng, tt.step)
		assert.Zero(t, end%step, "aligned")
	}
}

func TestAuth(t *testing.T) {
	var header atomic.Value
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		header.Store(r.Header.Get("Authorization"))
		reply(`{"status":"success","data":{"resultType":"scalar","result":[0,"1"]}}`)(w, r)
	})

	for auth, want := range map[string]string{
		"none":   "",
		"basic":  "Basic YW5uOmh1bnRlcjI=",
		"bearer": "Bearer t0ken",
	} {
		cfg["auth"] = auth
		cfg["username"] = "ann"
		cfg["password"] = "hunter2"
		cfg["token"] = "t0ken"
		_, err := Type{}.Test(context.Background(), cfg)
		require.NoError(t, err, auth)
		assert.Equal(t, want, header.Load(), auth)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		handler http.HandlerFunc
		want    string
	}{
		{func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"status":"error","errorType":"bad_data","error":"parse error"}`)
		}, "bad_data: parse error"},
		{func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}, "HTTP 503 Service Unavailable"},
		{reply(`<html>`), "invalid response"},
		{reply(`{"status":"success","data":{"resultType":"vector","result":[{"value":"x"}]}}`), "invalid response"},
		{reply(`{"status":"success","data":"` + strings.Repeat("x", datasource.MaxResponse) + `"}`), "larger than"},
	}
	for _, tt := range tests {
		p := model.Panel{
			Type:  model.PanelStat,
			Query: "x",
		}
		_, err := Type{}.Evaluate(context.Background(), stub(t, tt.handler), p, now)
		assert.ErrorContains(t, err, tt.want)
	}

	cfg := stub(t, nil)
	cfg["url"] = "http://127.0.0.1:1"
	_, err := Type{}.Test(context.Background(), cfg)
	assert.ErrorContains(t, err, "connection refused")
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	cfg := stub(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	cfg["timeout"] = "1s"
	start := time.Now()
	_, err := Type{}.Test(context.Background(), cfg)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var hits atomic.Int32
	count := func(http.ResponseWriter, *http.Request) { hits.Add(1) }
	other := httptest.NewServer(http.HandlerFunc(count))
	t.Cleanup(other.Close)
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	})

	cfg["auth"], cfg["token"] = "bearer", "t0ken"
	_, err := Type{}.Test(context.Background(), cfg)
	assert.ErrorContains(t, err, "redirect not followed: HTTP 307")
	assert.Zero(t, hits.Load())
}

func TestConnectionTest(t *testing.T) {
	var query string
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.FormValue("query")
		reply(`{"status":"success","data":{"resultType":"scalar","result":[0,"1"]}}`)(w, r)
	})

	detail, err := Type{}.Test(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "1", query)
	assert.Regexp(t, `^\d+ ms$`, detail)
}

func TestProxy(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	var upstream *http.Request
	var body string
	cfg := stub(t, func(w http.ResponseWriter, r *http.Request) {
		upstream = r
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		http.SetCookie(w, &http.Cookie{
			Name:  "upstream",
			Value: "x",
		})
		w.Header().Set("X-Upstream", "x")
		reply(`{"status":"success","data":["job"]}`)(w, r)
	})

	cfg["auth"], cfg["token"] = "bearer", "t0ken"
	proxy := func(
		method, path, query, reqBody string,
		headers ...string,
	) (*httptest.ResponseRecorder, error) {
		upstream = nil
		target := url.URL{
			Path:     "/api/admin/datasources/x/prometheus" + path,
			RawQuery: query,
		}
		r := httptest.NewRequest(
			method,
			target.String(),
			strings.NewReader(reqBody),
		)
		for i := 0; i < len(headers); i += 2 {
			r.Header.Set(headers[i], headers[i+1])
		}

		w := httptest.NewRecorder()
		return w, Type{}.ServeAdmin(w, r, cfg, path)
	}

	w, err := proxy(
		http.MethodGet,
		"/api/v1/labels",
		"match[]=up",
		"",
		"Cookie", "sitrep_session=secret",
		"Authorization", "Basic x",
		"Origin", "https://admin.example.com",
		"Referer", "https://admin.example.com/admin/",
		"Forwarded", "for=192.0.2.1",
		"X-Forwarded-For", "192.0.2.1",
		"X-Forwarded-Host", "admin.example.com",
		"Connection", "close",
		"Te", "trailers",
	)
	require.NoError(err)
	require.NotNil(upstream)
	assert.Equal(http.StatusOK, w.Code)
	assert.JSONEq(`{"status":"success","data":["job"]}`, w.Body.String())
	assert.Equal("application/json", w.Header().Get("Content-Type"))
	assert.Empty(w.Header().Values("Set-Cookie"))
	assert.Empty(w.Header().Get("X-Upstream"))
	assert.Equal("/api/v1/labels", upstream.URL.Path)
	assert.Equal("match[]=up", upstream.URL.RawQuery)
	assert.Equal("Bearer t0ken", upstream.Header.Get("Authorization"))
	for _, h := range []string{
		"Cookie", "Origin", "Referer", "Forwarded",
		"X-Forwarded-For", "X-Forwarded-Host", "Te",
	} {
		assert.Empty(upstream.Header.Get(h), h)
	}

	_, err = proxy(
		http.MethodPost,
		"/api/v1/series",
		"",
		"match%5B%5D=up",
		"Content-Type", "application/x-www-form-urlencoded",
	)
	require.NoError(err)
	assert.Equal(http.MethodPost, upstream.Method)
	assert.Equal("match%5B%5D=up", body)
	contentType := upstream.Header.Get("Content-Type")
	assert.Equal("application/x-www-form-urlencoded", contentType)

	_, err = proxy(http.MethodGet, "/api/v1/label/a b?/values", "", "")
	require.NoError(err)
	assert.Equal("/api/v1/label/a%20b%3F/values", upstream.URL.EscapedPath())

	for _, path := range []string{
		"/api/v1/metadata",
		"/api/v1/status/flags",
		"/api/v1/label/job/values",
	} {
		_, err = proxy(http.MethodGet, path, "", "")
		require.NoError(err, path)
	}

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/query"},
		{http.MethodGet, "/api/v1/admin/tsdb/delete_series"},
		{http.MethodGet, "/api/v1/label/../values"},
		{http.MethodGet, "/api/v1/label//values"},
		{http.MethodGet, "/api/v1/label/a/b/values"},
		{http.MethodGet, "/api/v1/labels/"},
		{http.MethodGet, "/federate"},
	} {
		_, err := proxy(c.method, c.path, "", "")
		assertAPIError(t, err, http.StatusNotFound, c.path)
		assert.Nil(upstream, "rejected requests never reach the backend")
	}

	for _, c := range []struct{ method, path, allow string }{
		{http.MethodPost, "/api/v1/metadata", "GET"},
		{http.MethodPost, "/api/v1/label/job/values", "GET"},
		{http.MethodDelete, "/api/v1/series", "GET, POST"},
		{http.MethodPut, "/api/v1/labels", "GET, POST"},
	} {
		w, err := proxy(c.method, c.path, "", "")
		assertAPIError(t, err, http.StatusMethodNotAllowed, c.path)
		assert.Equal(c.allow, w.Header().Get("Allow"))
		assert.Nil(upstream)
	}

	tooLarge := strings.Repeat("x", 1<<20+1)
	_, err = proxy(http.MethodPost, "/api/v1/series", "", tooLarge)
	assertAPIError(t, err, http.StatusRequestEntityTooLarge, "body limit")
	assert.Nil(upstream)

	cfg["url"] = "http://127.0.0.1:1"
	_, err = proxy(http.MethodGet, "/api/v1/labels", "", "")
	assertAPIError(t, err, http.StatusBadGateway, "unreachable backend")
}

func TestProxyPassesOnlySuccessfulAnswers(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	answer := func(status int, contentType, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}
	}

	proxy := func(handler http.HandlerFunc) (*httptest.ResponseRecorder, error) {
		target := "/api/admin/datasources/x/prometheus/api/v1/labels"
		r := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		return w, Type{}.ServeAdmin(w, r, stub(t, handler), "/api/v1/labels")
	}

	w, err := proxy(answer(http.StatusOK, "text/html", "<script>alert(1)</script>"))
	require.NoError(err)
	assert.Equal(http.StatusOK, w.Code)
	assert.Equal(
		"application/json",
		w.Header().Get("Content-Type"),
		"the upstream media type is never passed on",
	)
	assert.Equal("<script>alert(1)</script>", w.Body.String())

	handler := answer(http.StatusNonAuthoritativeInfo, "application/json", "[]")
	w, err = proxy(handler)
	require.NoError(err)
	assert.Equal(http.StatusNonAuthoritativeInfo, w.Code)

	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
	} {
		w, err := proxy(answer(status, "text/html", "<h1>Sign in</h1>"))
		assertAPIError(t, err, http.StatusBadGateway, http.StatusText(status))
		assert.ErrorContains(err, fmt.Sprintf("HTTP %d", status))
		assert.Empty(w.Body.String(), "nothing of the upstream answer is written")
		assert.Empty(w.Header().Get("Content-Type"))
	}

	oversized := `"` + strings.Repeat("x", datasource.MaxResponse) + `"`
	_, err = proxy(answer(http.StatusOK, "application/json", oversized))
	assertAPIError(t, err, http.StatusBadGateway, "oversized answer")
}

func assertAPIError(t *testing.T, err error, status int, msg string) {
	t.Helper()
	e, ok := errors.AsType[*apierr.Error](err)
	if assert.True(t, ok, msg) {
		assert.Equal(t, status, e.Status, msg)
	}
}

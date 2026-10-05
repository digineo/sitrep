package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/process"
)

// fakeType is a data source type that exists only in tests: it proves that
// polling, previews, status and caching work for any type. Its queries are
// numbers separated by spaces, other words are ignored. Each number becomes
// a labeled sample of an instant, or a series with that value at three
// times. A query containing "fail" fails, one containing "large" reports a
// response just over the size that gets a warning.
type fakeType struct {
	panelTypes []string
}

func (fakeType) Fields() []datasource.Field {
	return []datasource.Field{
		{
			Name:     "endpoint",
			Kind:     datasource.KindText,
			Required: true,
		},
		{
			Name: "url",
			Kind: datasource.KindURL,
		},
		{
			Name: "token",
			Kind: datasource.KindSecret,
		},
	}
}

func (t fakeType) PanelTypes() []string { return t.panelTypes }

func (fakeType) Evaluate(
	_ context.Context,
	cfg datasource.Config,
	p model.Panel,
	now time.Time,
) (datasource.Result, error) {
	if strings.Contains(p.Query, "fail") {
		return datasource.Result{}, errors.New("cannot reach " + cfg["endpoint"])
	}

	var values []float64
	for _, word := range strings.Fields(p.Query) {
		if v, err := strconv.ParseFloat(word, 64); err == nil {
			values = append(values, v)
		}
	}

	var r datasource.Result
	if strings.Contains(p.Query, "large") {
		r.Bytes = process.LargeResponse + 1
	}

	if p.Type == model.PanelTimeseries {
		end := now.Truncate(time.Minute)
		r.Times = []time.Time{end.Add(-2 * time.Minute), end.Add(-time.Minute), end}
		for i, v := range values {
			r.Series = append(r.Series, datasource.Series{
				Labels: map[string]string{
					"name":   "series-" + strconv.Itoa(i),
					"secret": "SECRET-LABEL",
				},
				Values: []float64{v, v, v},
			})
		}
		return r, nil
	}

	for i, v := range values {
		r.Samples = append(r.Samples, datasource.Sample{
			Labels: map[string]string{"i": strconv.Itoa(i)},
			Value:  v,
		})
	}
	return r, nil
}

func (fakeType) Test(_ context.Context, cfg datasource.Config) (string, error) {
	if cfg["token"] == "" {
		return "", errors.New("no token")
	}
	return "token works", nil
}

func (fakeType) Summary(cfg datasource.Config) string { return cfg["endpoint"] }

// ServeAdmin echoes the request.
func (fakeType) ServeAdmin(
	w http.ResponseWriter,
	r *http.Request,
	cfg datasource.Config,
	path string,
) error {
	if path == "/missing" {
		return apierr.New(http.StatusNotFound, apierr.NotFound)
	}
	body, _ := io.ReadAll(r.Body)
	_, _ = io.WriteString(w, r.Method+" "+path+" "+string(body)+" "+cfg["token"])
	return nil
}

func init() {
	datasource.Register("fake", fakeType{panelTypes: []string{
		model.PanelStat,
		model.PanelStatus,
		model.PanelTimeseries,
	}})
	datasource.Register("fake-instant", fakeType{panelTypes: []string{
		model.PanelStat,
		model.PanelStatus,
	}})
}

// signIn signs in the identity, given as JSON, for the following admin
// requests.
func (f *fixture) signIn(identity string) {
	f.t.Helper()
	r := httptest.NewRequest(
		http.MethodPost,
		"http://status.example.com/auth/test/login",
		strings.NewReader(identity),
	)
	r.Header.Set("Origin", "http://status.example.com")
	r.Header.Set("Content-Type", "application/json")
	cookies := f.do(r).Result().Cookies()
	require.Len(f.t, cookies, 1)
	f.session = cookies[0]
}

// admin sends an admin API request with a JSON body, signed in, by default
// as Ann, the owner.
func (f *fixture) admin(method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	if f.session == nil {
		f.signIn("{}")
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(f.t, err)
		reader = bytes.NewReader(raw)
	}

	r := httptest.NewRequest(method, "http://status.example.com"+path, reader)
	r.Header.Set("Origin", "http://status.example.com")
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(f.session)
	return f.do(r)
}

// decode decodes a response body, requiring the status.
func decode[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	require.Equal(t, status, w.Code, w.Body.String())
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v))
	return v
}

type errorBody struct {
	Error struct {
		Code    string         `json:"code"`
		Fields  []apierr.Field `json:"fields"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

// fieldCodes returns a validation error's fields by path.
func fieldCodes(
	t *testing.T,
	w *httptest.ResponseRecorder,
	status int,
) map[string]string {
	t.Helper()
	body := decode[errorBody](t, w, status)
	out := map[string]string{}
	for _, f := range body.Error.Fields {
		out[f.Path] = f.Code
	}
	return out
}

type obj = map[string]any

func (f *fixture) createDataSource(
	name, typ string,
	config map[string]string,
) string {
	f.t.Helper()
	w := f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name":   name,
		"type":   typ,
		"config": config,
	})
	return decode[dataSourceView](f.t, w, http.StatusCreated).ID
}

func (f *fixture) createSite(slug string, langs ...string) string {
	f.t.Helper()
	name := model.Text{}
	for _, l := range langs {
		name[l] = "Site " + l
	}

	site := obj{
		"name": name,
		"languages": obj{
			"enabled": langs,
			"primary": langs[0],
		},
		"route": obj{
			"mode": "path",
			"slug": slug,
		},
	}
	w := f.admin(http.MethodPost, "/api/admin/sites", site)
	return decode[model.Site](f.t, w, http.StatusCreated).ID
}

func (f *fixture) createPanel(site string, panel obj) string {
	f.t.Helper()
	w := f.admin(http.MethodPost, "/api/admin/sites/"+site+"/panels", panel)
	return decode[model.Panel](f.t, w, http.StatusCreated).ID
}

func TestDataSourceAPI(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.admin(http.MethodGet, "/api/admin/datasource-types", nil)
	types := decode[[]typeView](t, w, http.StatusOK)
	require.Len(types, 2)
	assert.Equal("fake", types[0].ID)
	assert.Equal(fakeType{}.Fields(), types[0].Fields)
	assert.Equal([]string{"stat", "status"}, types[1].PanelTypes)

	w = f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name":   " ",
		"type":   "fake",
		"config": obj{},
	})
	want := map[string]string{
		"name":            "required",
		"config.endpoint": "required",
	}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name":   "x",
		"type":   "nope",
		"config": obj{},
	})
	want = map[string]string{"type": "invalid_value"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name": "Main",
		"type": "fake",
		"config": obj{
			"endpoint": "internal:9090",
			"token":    "s3cr3t-t0ken",
		},
	})
	assert.NotContains(w.Body.String(), "s3cr3t-t0ken")
	ds := decode[dataSourceView](t, w, http.StatusCreated)
	created := dataSourceView{
		ID:      ds.ID,
		Name:    "Main",
		Type:    "fake",
		Config:  map[string]string{"endpoint": "internal:9090"},
		Secrets: []string{"token"},
		Summary: "internal:9090",
		Usable:  true,
	}
	assert.Equal(created, ds)

	w = f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name":   "MAIN",
		"type":   "fake",
		"config": obj{"endpoint": "x"},
	})
	want = map[string]string{"name": "name_taken"}
	assert.Equal(want, fieldCodes(t, w, http.StatusConflict))

	w = f.admin(http.MethodGet, "/api/admin/datasources", nil)
	assert.NotContains(w.Body.String(), "s3cr3t-t0ken")
	list := decode[[]dataSourceView](t, w, http.StatusOK)
	assert.Equal([]dataSourceView{ds}, list)
	raw, err := os.ReadFile(f.dbPath)
	require.NoError(err)
	leaked := bytes.Contains(raw, []byte("s3cr3t-t0ken"))
	assert.False(leaked, "the database never holds secrets in plain text")

	w = f.admin(http.MethodPut, "/api/admin/datasources/"+ds.ID, obj{
		"name":   "Main",
		"config": obj{"endpoint": "other:9090"},
	})
	ds = decode[dataSourceView](t, w, http.StatusOK)
	assert.Equal([]string{"token"}, ds.Secrets, "an omitted secret keeps its value")

	w = f.admin(http.MethodPost, "/api/admin/datasources/"+ds.ID+"/test", nil)
	works := testResult{
		OK:     true,
		Detail: "token works",
	}
	assert.Equal(works, decode[testResult](t, w, http.StatusOK))

	w = f.admin(http.MethodPost, "/api/admin/datasources/test", obj{
		"id":     ds.ID,
		"type":   "fake",
		"config": obj{"endpoint": "unsaved"},
	})
	assert.Equal(
		works,
		decode[testResult](t, w, http.StatusOK),
		"unsaved tests use the stored secrets that are not replaced",
	)

	w = f.admin(http.MethodPost, "/api/admin/datasources/test", obj{
		"type":   "fake",
		"config": obj{"endpoint": "unsaved"},
	})
	noToken := testResult{Error: "no token"}
	assert.Equal(noToken, decode[testResult](t, w, http.StatusOK))

	w = f.admin(http.MethodPut, "/api/admin/datasources/"+ds.ID, obj{
		"name":   "Main",
		"type":   "fake-instant",
		"config": obj{"endpoint": "x"},
	})
	want = map[string]string{"type": "invalid_value"}
	assert.Equal(
		want,
		fieldCodes(t, w, http.StatusBadRequest),
		"the type is immutable",
	)

	w = f.admin(http.MethodPut, "/api/admin/datasources/"+ds.ID, obj{
		"name": "Main",
		"config": obj{
			"endpoint": "x",
			"url":      "https://a.example.com",
		},
	})
	ds = decode[dataSourceView](t, w, http.StatusOK)
	assert.Empty(ds.Secrets, "a changed URL discards the stored secrets")

	w = f.admin(http.MethodPut, "/api/admin/datasources/"+ds.ID, obj{
		"name": "Main",
		"config": obj{
			"endpoint": "x",
			"url":      "https://a.example.com",
			"token":    "t",
		},
	})
	ds = decode[dataSourceView](t, w, http.StatusOK)
	w = f.admin(http.MethodPost, "/api/admin/datasources/test", obj{
		"id":   ds.ID,
		"type": "fake",
		"config": obj{
			"endpoint": "x",
			"url":      "https://b.example.com",
		},
	})
	assert.Equal(
		noToken,
		decode[testResult](t, w, http.StatusOK),
		"unsaved tests against another URL don't use the stored secrets",
	)

	w = f.admin(http.MethodPut, "/api/admin/datasources/"+ds.ID, obj{
		"name": "Main",
		"config": obj{
			"endpoint": "x",
			"url":      "https://a.example.com",
			"token":    "",
		},
	})
	ds = decode[dataSourceView](t, w, http.StatusOK)
	assert.Empty(ds.Secrets, "an empty secret clears it")
	w = f.admin(http.MethodPost, "/api/admin/datasources/"+ds.ID+"/test", nil)
	assert.Equal(noToken, decode[testResult](t, w, http.StatusOK))

	w = f.admin(http.MethodDelete, "/api/admin/datasources/"+ds.ID, nil)
	assert.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodGet, "/api/admin/datasources/"+ds.ID, nil)
	assert.Equal(http.StatusNotFound, w.Code)
}

func TestDataSourceInUse(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	site := f.createSite("shop", "en")
	panel := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "Up"},
		"datasource": ds,
		"query":      "1",
	})

	w := f.admin(http.MethodDelete, "/api/admin/datasources/"+ds, nil)
	body := decode[errorBody](t, w, http.StatusConflict)
	assert.Equal("datasource_in_use", body.Error.Code)
	want := map[string]any{
		"panels": 1.0,
		"sites":  []any{site},
	}
	assert.Equal(want, body.Error.Details)

	w = f.admin(http.MethodDelete, "/api/admin/sites/"+site+"/panels/"+panel, nil)
	require.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodDelete, "/api/admin/datasources/"+ds, nil)
	assert.Equal(http.StatusNoContent, w.Code)
}

func TestSecretKeyMissing(t *testing.T) {
	f := newFixture(t)
	f.srv.(*Server).cfg.SecretKey = nil
	w := f.admin(http.MethodPost, "/api/admin/datasources", obj{
		"name": "x",
		"type": "fake",
		"config": obj{
			"endpoint": "x",
			"token":    "t",
		},
	})
	want := map[string]string{"config.token": "secret_key_missing"}
	assert.Equal(t, want, fieldCodes(t, w, http.StatusBadRequest))

	f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
}

func TestUnusableDataSource(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := &model.DataSource{
		ID:   "0199a000-0000-7000-8000-000000000001",
		Name: "Old key",
		Type: "fake",
	}
	cfg := map[string]string{
		"endpoint": "x",
		"token":    "t",
	}
	var errs apierr.Fields
	require.NoError(datasource.Apply(&errs, ds, cfg, bytes.Repeat([]byte{1}, 32)))
	require.Empty(errs)
	require.NoError(f.db.CreateDataSource(ds))

	w := f.admin(http.MethodGet, "/api/admin/datasources", nil)
	list := decode[[]dataSourceView](t, w, http.StatusOK)
	assert.False(list[0].Usable)
	w = f.admin(http.MethodGet, "/api/admin/datasources/"+ds.ID+"/fake/echo", nil)
	assert.Equal(http.StatusConflict, w.Code)

	site := f.createSite("shop", "en")
	panel := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "Up"},
		"datasource": ds.ID,
		"query":      "1",
	})
	unusable := func() bool {
		w := f.admin(http.MethodGet, "/api/admin/sites/"+site+"/preview", nil)
		p := decode[payload](t, w, http.StatusOK)
		return p.Panels[panel].Error == datasource.ErrUnusable.Error()
	}

	require.Eventually(unusable, 5*time.Second, 10*time.Millisecond)
	w = f.get("status.example.com", "/api/public/sites/"+site)
	require.Equal(http.StatusOK, w.Code, "public serving continues")
	assert.NotContains(w.Body.String(), "decrypted")
}

func TestDataSourceRoutes(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{
		"endpoint": "x",
		"token":    "t0ken",
	})
	base := "/api/admin/datasources/" + ds

	w := f.admin(http.MethodGet, base+"/fake/api/v1/labels?match[]=up", nil)
	assert.Equal(http.StatusOK, w.Code)
	assert.Equal("GET /api/v1/labels  t0ken", w.Body.String())
	w = f.admin(http.MethodGet, base+"/fake/missing", nil)
	assert.Equal(http.StatusNotFound, w.Code)
	w = f.admin(http.MethodGet, base+"/fake-instant/x", nil)
	assert.Equal(http.StatusNotFound, w.Code, "the type must match")
	w = f.admin(http.MethodGet, "/api/admin/datasources/unknown/fake/x", nil)
	assert.Equal(http.StatusNotFound, w.Code)

	form := func(origin, contentType string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(
			http.MethodPost,
			"http://status.example.com"+base+"/fake/api/v1/series",
			strings.NewReader("match%5B%5D=up"),
		)
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", contentType)
		r.AddCookie(f.session)
		return f.do(r)
	}

	w = form("http://status.example.com", "application/x-www-form-urlencoded")
	assert.Equal(http.StatusOK, w.Code)
	assert.Equal("POST /api/v1/series match%5B%5D=up t0ken", w.Body.String())
	w = form("http://evil.example", "application/x-www-form-urlencoded")
	assert.Equal(http.StatusForbidden, w.Code)
	w = form("http://status.example.com", "text/plain")
	assert.Equal(http.StatusUnsupportedMediaType, w.Code)

	w = f.admin(http.MethodPost, base+"/test", nil)
	assert.Equal(
		http.StatusOK,
		w.Code,
		"the test route is not taken for a type route",
	)
	r := httptest.NewRequest(
		http.MethodGet,
		"http://status.example.com"+base+"/fake/x",
		nil,
	)
	assert.Equal(http.StatusUnauthorized, f.do(r).Code)
}

func TestSiteAPI(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	w := f.admin(http.MethodPost, "/api/admin/sites", obj{
		"languages": obj{
			"enabled": []string{"en"},
			"primary": "en",
		},
		"route": obj{
			"mode": "path",
			"slug": "admin",
		},
		"timezone": "Mars/Olympus",
	})
	want := map[string]string{
		"name.en":    "required",
		"route.slug": "slug_reserved",
		"timezone":   "invalid_timezone",
	}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, "/api/admin/sites", obj{
		"name": obj{"en": "Shop"},
		"languages": obj{
			"enabled": []string{"en"},
			"primary": "en",
		},
		"route": obj{
			"mode": "path",
			"slug": "acme",
		},
	})
	want = map[string]string{"route.slug": "route_conflict"}
	assert.Equal(want, fieldCodes(t, w, http.StatusConflict))

	id := f.createSite("shop", "en", "de")
	w = f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](t, w, http.StatusOK)
	assert.Equal("UTC", site.Timezone)
	assert.Equal("inherit", site.Theme)
	assert.Equal(http.StatusOK, f.get("status.example.com", "/shop/en/").Code)

	site.Route = model.Route{
		Mode: model.RouteSubdomain,
		Slug: "shop",
	}
	site.Timezone = "Europe/Berlin"
	site.Name = model.Text{"en": "Shop"}
	w = f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	updated := decode[model.Site](t, w, http.StatusOK)
	assert.Equal("Europe/Berlin", updated.Timezone)
	assert.Equal(site.CreatedAt, updated.CreatedAt)
	assert.Equal(http.StatusNotFound, f.get("status.example.com", "/shop/en/").Code)
	assert.Equal(http.StatusOK, f.get("shop.status.example.com", "/en/").Code)

	w = f.admin(http.MethodGet, "/api/admin/sites", nil)
	sites := decode[[]siteSummary](t, w, http.StatusOK)
	require.Len(sites, 4)
	var sum siteSummary
	for _, s := range sites {
		if s.ID == id {
			sum = s
		}
	}

	status := siteStatus{
		Overall:   "operational",
		Panels:    "operational",
		Incidents: "operational",
	}
	assert.Equal(status, sum.Status)
	assert.Equal(1, sum.Missing, "the name lacks German")

	w = f.admin(http.MethodDelete, "/api/admin/sites/"+id, nil)
	require.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	assert.Equal(http.StatusNotFound, w.Code)
	assert.Equal(http.StatusNotFound, f.get("shop.status.example.com", "/en/").Code)
}

func TestPanelAPI(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	instant := f.createDataSource(
		"Instant",
		"fake-instant",
		map[string]string{"endpoint": "x"},
	)
	site := f.createSite("shop", "en", "de")
	panels := "/api/admin/sites/" + site + "/panels"

	w := f.admin(http.MethodPost, panels, obj{"type": "stat"})
	want := map[string]string{
		"title.en":   "required",
		"datasource": "required",
		"query":      "required",
	}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, panels, obj{
		"type":       "stat",
		"title":      obj{"en": "x"},
		"datasource": "missing",
		"query":      "1",
	})
	want = map[string]string{"datasource": "not_found"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, panels, obj{
		"type":       "timeseries",
		"title":      obj{"en": "x"},
		"datasource": instant,
		"query":      "1",
		"range":      "1h",
	})
	want = map[string]string{"datasource": "datasource_unsupported_type"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, "/api/admin/sites/missing/panels", obj{})
	assert.Equal(http.StatusNotFound, w.Code)

	a := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "A"},
		"datasource": ds,
		"query":      "1",
		"order":      9,
		"revision":   9,
		"range":      "1h",
	})
	b := f.createPanel(site, obj{
		"type": "status",
		"title": obj{
			"en": "B",
			"de": "B",
		},
		"datasource": instant,
		"query":      "1",
	})
	w = f.admin(http.MethodGet, panels, nil)
	got := decode[[]model.Panel](t, w, http.StatusOK)
	require.Len(got, 2)
	assert.Equal(a, got[0].ID)
	assert.Equal(int64(1), got[0].Revision)
	assert.Empty(got[0].Range, "options of other types are dropped")

	order := "/api/admin/sites/" + site + "/panel-order"
	w = f.admin(http.MethodPut, order, obj{"panels": []string{b, a}})
	require.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodGet, panels, nil)
	got = decode[[]model.Panel](t, w, http.StatusOK)
	assert.Equal([]string{b, a}, []string{got[0].ID, got[1].ID})
	w = f.admin(http.MethodPut, order, obj{"panels": []string{a}})
	assert.Equal(http.StatusBadRequest, w.Code)

	w = f.admin(http.MethodPut, panels+"/"+a, obj{
		"type":       "stat",
		"title":      obj{"en": "A2"},
		"datasource": ds,
		"query":      "2",
	})
	p := decode[model.Panel](t, w, http.StatusOK)
	assert.Equal(int64(2), p.Revision)
	assert.Equal(1, p.Order)

	w = f.admin(http.MethodGet, "/api/admin/sites", nil)
	sites := decode[[]siteSummary](t, w, http.StatusOK)
	for _, s := range sites {
		if s.ID == site {
			assert.Equal(1, s.Missing, "panel A lacks German")
		}
	}

	w = f.admin(http.MethodDelete, panels+"/"+a, nil)
	require.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodGet, panels+"/"+a, nil)
	assert.Equal(http.StatusNotFound, w.Code)
}

func TestPanelPreview(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource(
		"Main",
		"fake",
		map[string]string{"endpoint": "internal:9090"},
	)
	site := f.createSite("shop", "en", "de")
	path := "/api/admin/sites/" + site + "/panel-preview"
	preview := func(panel obj, lang string) previewResult {
		w := f.admin(http.MethodPost, path, obj{
			"panel": panel,
			"lang":  lang,
		})
		return decode[previewResult](t, w, http.StatusOK)
	}

	panel := obj{
		"type":       "stat",
		"datasource": ds,
		"query":      "2 3",
		"reduce":     "sum",
	}
	res := preview(panel, "en")
	want := previewResult{
		Data:     obj{"value": 5.0},
		Warnings: []process.Warning{},
	}
	assert.Equal(want, res, "previews need no title")

	query := strings.Repeat("1 ", 23) + "large"
	panel = obj{
		"type": "timeseries",
		"title": obj{
			"en": "Load",
			"de": "Last",
		},
		"datasource": ds,
		"query":      query,
		"range":      "1h",
	}
	res = preview(panel, "de")
	warnings := []process.Warning{
		{
			Code:  "series_dropped",
			Count: 3,
		},
		{
			Code: "response_large",
			Size: process.LargeResponse + 1,
		},
	}
	assert.Equal(warnings, res.Warnings)
	series := res.Data.(obj)["series"].([]any)
	require.Len(series, 20)
	assert.Equal("Last #1", series[0].(obj)["name"])
	assert.NotContains(series[0], "labels")

	panel = obj{
		"type":       "stat",
		"datasource": ds,
		"query":      "fail",
	}
	res = preview(panel, "en")
	assert.Equal("cannot reach internal:9090", res.Error)
	assert.Nil(res.Data)

	w := f.admin(http.MethodPost, path, obj{"panel": obj{
		"type":       "stat",
		"datasource": ds,
	}})
	codes := map[string]string{"query": "required"}
	assert.Equal(codes, fieldCodes(t, w, http.StatusBadRequest))
}

func TestSitePreviewWarnings(t *testing.T) {
	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	site := f.createSite("shop", "en")
	panel := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "Big"},
		"datasource": ds,
		"query":      "1 large",
		"refresh":    "1d",
	})
	f.awaitFresh(site)

	w := f.admin(http.MethodGet, "/api/admin/sites/"+site+"/preview", nil)
	p := decode[payload](t, w, http.StatusOK)
	want := []process.Warning{{
		Code: "response_large",
		Size: process.LargeResponse + 1,
	}}
	assert.Equal(t, want, p.Panels[panel].Warnings)
	w = f.get("status.example.com", "/api/public/sites/"+site)
	body := w.Body.String()
	assert.NotContains(t, body, "warnings", "warnings are for admins only")
}

// publicPayload fetches a site's public payload.
func (f *fixture) publicPayload(
	site, query string,
) (payload, map[string]json.RawMessage) {
	f.t.Helper()
	require := require.New(f.t)
	assert := assert.New(f.t)

	w := f.get("status.example.com", "/api/public/sites/"+site+query)
	require.Equal(http.StatusOK, w.Code, w.Body.String())
	assert.Equal("public, max-age=10", w.Header().Get("Cache-Control"))
	var p payload
	var raw map[string]json.RawMessage
	require.NoError(json.Unmarshal(w.Body.Bytes(), &p))
	require.NoError(json.Unmarshal(w.Body.Bytes(), &raw))
	return p, raw
}

// awaitFresh waits until all of the site's panels have been polled and
// returns a payload fetched afterwards. Its cursor covers all polls so far,
// while the payload that first saw them fresh may have read the cursor
// before the last poll finished.
func (f *fixture) awaitFresh(site string) payload {
	f.t.Helper()
	fresh := func() bool {
		p, _ := f.publicPayload(site, "")
		for _, pp := range p.Panels {
			if pp.State != "fresh" {
				return false
			}
		}
		return true
	}

	require.Eventually(f.t, fresh, 5*time.Second, 10*time.Millisecond)
	p, _ := f.publicPayload(site, "")
	return p
}

func TestPublicPayload(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	site := f.createSite("shop", "en", "de")
	status := f.createPanel(site, obj{
		"type": "status",
		"title": obj{
			"en": "API",
			"de": "Schnittstelle",
		},
		"datasource": ds,
		"query":      "0",
		"refresh":    "1d",
		"thresholds": []obj{{
			"op":    "<",
			"value": 1,
			"state": "down",
		}},
	})
	stat := f.createPanel(site, obj{
		"type":        "stat",
		"title":       obj{"en": "Users"},
		"description": obj{"en": "Signed in"},
		"unit":        obj{"de": "Personen"},
		"datasource":  ds,
		"query":       "5 7",
		"reduce":      "sum",
		"refresh":     "1d",
	})
	chart := f.createPanel(site, obj{
		"type":       "timeseries",
		"title":      obj{"en": "Load"},
		"datasource": ds,
		"query":      "1 2",
		"refresh":    "1d",
		"range":      "2h",
		"legend":     obj{"en": "{{ name }}"},
	})

	f.awaitFresh(site)
	p, _ := f.publicPayload(site, "?lang=de")
	assert.Equal("down", p.Status)
	require.NotNil(p.Site)
	two := 2
	zero := 0
	want := sitePayload{
		Name:      "Site de",
		Theme:     "system",
		Languages: []string{"en", "de"},
		Timezone:  "UTC",
		Panels: []panelInfo{
			{
				ID:    status,
				Type:  "status",
				Title: "Schnittstelle",
			},
			{
				ID:          stat,
				Type:        "stat",
				Title:       "Users",
				Description: "Signed in",
				Unit:        "Personen",
				Decimals:    &zero,
			},
			{
				ID:       chart,
				Type:     "timeseries",
				Title:    "Load",
				Decimals: &two,
				Style:    "line",
				Range:    7200,
			},
		},
	}
	assert.Equal(want, *p.Site)
	assert.Equal(obj{"state": "down"}, p.Panels[status].Data)
	assert.Equal(obj{"value": 12.0}, p.Panels[stat].Data)
	data := p.Panels[chart].Data.(obj)
	assert.Len(data["times"], 3)
	series := []any{
		obj{
			"name":   "series-0",
			"values": []any{1.0, 1.0, 1.0},
		},
		obj{
			"name":   "series-1",
			"values": []any{2.0, 2.0, 2.0},
		},
	}
	assert.Equal(series, data["series"])
	assert.Empty(p.Panels[stat].Error)

	p, _ = f.publicPayload(site, "?lang=fr")
	assert.Equal(
		"Site en",
		p.Site.Name,
		"languages that are not enabled fall back to the primary one",
	)

	w := f.get("status.example.com", "/api/public/sites/not-a-uuid")
	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Empty(w.Header().Get("Cache-Control"))
	unknown := "/api/public/sites/0199a000-0000-7000-8000-000000000001"
	assert.Equal(http.StatusNotFound, f.get("status.example.com", unknown).Code)
	w = f.get("unknown.example", "/api/public/sites/"+site)
	assert.Equal(http.StatusOK, w.Code, "the public API answers on every host")
}

func TestChangeCursor(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	site := f.createSite("shop", "en")
	a := f.createPanel(site, obj{
		"type":       "status",
		"title":      obj{"en": "A"},
		"datasource": ds,
		"query":      "1",
		"refresh":    "1d",
	})
	b := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "B"},
		"datasource": ds,
		"query":      "1",
		"refresh":    "1d",
	})
	full := f.awaitFresh(site)
	require.NotNil(full.Site)
	require.Len(full.Panels, 2)

	_, raw := f.publicPayload(site, "?since="+full.Cursor)
	assert.Equal(
		[]string{"cursor", "status"},
		keys(raw),
		"an unchanged state gives only the cursor and the status",
	)

	for _, since := range []string{
		"garbage",
		"0123456789abcdef.1",
		full.Cursor + "0",
	} {
		p, _ := f.publicPayload(site, "?since="+since)
		assert.NotNil(p.Site, since)
		assert.Len(
			p.Panels,
			2,
			"malformed cursors and cursors of other processes give everything: %s",
			since,
		)
	}

	// Polled every 50ms, panel C changes on its own.
	c := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "C"},
		"datasource": ds,
		"query":      "3",
	})
	p, _ := f.publicPayload(site, "?since="+full.Cursor)
	require.NotNil(p.Site, "a new panel changes the site section")
	assert.Len(p.Site.Panels, 3)
	p = f.awaitFresh(site)
	cursor := p.Cursor
	changed := func() bool {
		p, _ = f.publicPayload(site, "?since="+cursor)
		return len(p.Panels) > 0
	}

	require.Eventually(changed, 5*time.Second, 10*time.Millisecond)
	assert.Nil(p.Site)
	assert.Equal(
		[]string{c},
		keys(p.Panels),
		"new data of a single panel returns only that panel",
	)

	w := f.admin(http.MethodDelete, "/api/admin/sites/"+site+"/panels/"+c, nil)
	require.Equal(http.StatusNoContent, w.Code)
	f.admin(http.MethodPut, "/api/admin/sites/"+site+"/panels/"+a, obj{
		"type":       "status",
		"title":      obj{"en": "A2"},
		"datasource": ds,
		"query":      "1",
		"refresh":    "1d",
	})
	p = f.awaitFresh(site)
	p, _ = f.publicPayload(site, "?since="+cursor)
	require.NotNil(p.Site)
	assert.Equal(
		"A2",
		p.Site.Panels[0].Title,
		"site changes return the site section",
	)
	assert.Equal(
		[]string{a},
		keys(p.Panels),
		"a changed panel returns with new data, the others do not",
	)
	assert.NotContains(p.Panels, b)

	cursor = p.Cursor
	w = f.admin(http.MethodGet, "/api/admin/settings", nil)
	settings := decode[model.Settings](t, w, http.StatusOK)
	settings.DefaultTheme = "dark"
	w = f.admin(http.MethodPut, "/api/admin/settings", settings)
	require.Equal(http.StatusOK, w.Code)
	p, _ = f.publicPayload(site, "?since="+cursor)
	require.NotNil(p.Site, "instance settings changes return the site section")
	assert.Equal("dark", p.Site.Theme)
	assert.Empty(p.Panels)
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestPublicLeak fills every admin-only field with a marker and checks that
// no public response contains any of them.
func TestPublicLeak(t *testing.T) {
	f := newFixture(t)
	ds := f.createDataSource("SECRET-DS-NAME", "fake", map[string]string{
		"endpoint": "https://SECRET-HOST.internal",
		"token":    "SECRET-TOKEN",
	})
	site := f.createSite("shop", "en", "de")
	f.createPanel(site, obj{
		"type":       "status",
		"title":      obj{"en": "A"},
		"datasource": ds,
		"query":      "1 SECRET-QUERY",
		"refresh":    "17h",
		"reduce":     "max",
		"thresholds": []obj{{
			"op":    "<",
			"value": 31337.25,
			"state": "down",
		}},
	})
	f.createPanel(site, obj{
		"type":       "timeseries",
		"title":      obj{"en": "B"},
		"datasource": ds,
		"query":      "1 SECRET-QUERY",
		"range":      "1h",
	})
	failing := f.createPanel(site, obj{
		"type":       "stat",
		"title":      obj{"en": "C"},
		"datasource": ds,
		"query":      "fail SECRET-QUERY",
	})
	ready := func() bool {
		w := f.admin(http.MethodGet, "/api/admin/sites/"+site+"/preview", nil)
		p := decode[payload](t, w, http.StatusOK)
		polled := 0
		for _, pp := range p.Panels {
			if pp.State == "fresh" {
				polled++
			}
		}
		return p.Panels[failing].Error != "" && polled == 2
	}

	require.Eventually(t, ready, 5*time.Second, 10*time.Millisecond)

	inc := f.createIncident(site, obj{"en": "Outage"}, obj{
		"status":      "active",
		"description": obj{"en": "[details](javascript:SECRET-SOURCE)"},
	})
	incident := "/api/admin/sites/" + site + "/incidents/" + inc.ID
	f.admin(http.MethodPut, incident+"/updates/"+inc.Updates[0].ID, obj{
		"status":      "active",
		"description": obj{"en": "**bold** [x](javascript:SECRET-SOURCE)"},
	})

	source := obj{"en": "**bold** [x](javascript:SECRET-SOURCE)"}
	legal := obj{
		"imprint": obj{
			"mode": "text",
			"text": source,
		},
		"privacy": obj{"mode": "none"},
	}
	f.putLegal([]string{"en", "de"}, legal, source)
	f.putSiteLegal(site, model.Legal{Privacy: model.LegalPage{
		Mode: model.LegalText,
		Text: model.Text{"en": "**bold** [x](javascript:SECRET-SOURCE)"},
	}})

	markers := []string{
		"SECRET", "31337", `"fake"`, "17h", "max", "internal", "Ann", "ann",
		`"author"`, "editedBy", "editedAt", "displayName", "**",
	}
	check := func(paths ...string) {
		for _, path := range paths {
			body := f.get("status.example.com", path).Body.String()
			for _, m := range markers {
				assert.NotContains(t, body, m, path)
			}
		}
	}

	check(
		"/api/public/sites/"+site,
		"/api/public/sites/"+site+"?lang=de",
		"/shop/en/",
		"/shop/de/",
		"/api/public/sites/"+site+"/incidents",
		"/api/public/sites/"+site+"/incidents/"+inc.ID,
		"/shop/en/feed.atom",
		"/shop/de/incidents.json",
		"/shop/en/privacy",
		"/api/public/sites/"+site+"/legal/privacy",
		"/api/public/sites/"+site+"/legal/imprint",
		"/api/public/landing",
		"/api/public/legal",
		"/api/public/legal/imprint",
		"/en/imprint",
	)

	w := f.admin(http.MethodGet, "/api/admin/sites/"+site, nil)
	stored := decode[model.Site](t, w, http.StatusOK)
	stored.Availability = model.AvailabilityOffline
	w = f.admin(http.MethodPut, "/api/admin/sites/"+site, stored)
	require.Equal(t, http.StatusOK, w.Code)
	check(
		"/api/public/sites/"+site,
		"/shop/en/",
		"/api/public/sites/"+site+"/incidents/"+inc.ID,
		"/shop/en/feed.atom",
	)
}

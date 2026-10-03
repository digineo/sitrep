package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/model"
)

var fakeConfig = map[string]string{"endpoint": "x"}

// yamlRequest sends a YAML body to the admin API, signed in.
func (f *fixture) yamlRequest(
	method, path, body string,
) *httptest.ResponseRecorder {
	f.t.Helper()
	f.admin(http.MethodGet, "/api/admin/settings", nil) // signs in
	target := "http://status.example.com" + path
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Origin", "http://status.example.com")
	r.Header.Set("Content-Type", "application/yaml")
	r.AddCookie(f.session)
	return f.do(r)
}

func (f *fixture) export(site string) string {
	f.t.Helper()
	w := f.admin(http.MethodGet, "/api/admin/sites/"+site+"/export", nil)
	require.Equal(f.t, http.StatusOK, w.Code, w.Body.String())
	return w.Body.String()
}

// fullSite creates a site with every setting and a panel of each type.
func (f *fixture) fullSite(main, instant string) string {
	f.t.Helper()
	id := f.createSite("shop", "en", "de")
	get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](f.t, get, http.StatusOK)
	site.Name = model.Text{
		"en": "Shop",
		"de": "Laden",
		"fr": "Boutique",
	}
	site.Timezone = "Europe/Berlin"
	site.Route = model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.shop.example",
	}
	site.Theme = "dark"
	site.BrandColor = "#112233"
	site.Logo = testLogo
	site.Legal = model.Legal{
		Imprint: model.LegalPage{
			Mode: model.LegalText,
			Text: model.Text{
				"en": "**Shop Inc.**\n\nMain Street 1",
				"de": "**Laden GmbH**",
			},
		},
		Privacy: model.LegalPage{
			Mode: model.LegalURL,
			URL:  model.Text{"en": "https://shop.example/privacy"},
		},
	}
	site.AllowedOrigins = []string{"https://www.shop.example"}
	site.IncidentRetentionDays = 30
	site.Availability = model.AvailabilityOffline
	put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	require.Equal(f.t, http.StatusOK, put.Code)

	f.createPanel(id, obj{
		"type": "status",
		"title": obj{
			"en": "API",
			"de": "Schnittstelle",
		},
		"description": obj{"en": "Public API"},
		"datasource":  instant,
		"query":       "up == 1",
		"refresh":     "90s",
		"reduce":      "min",
		"thresholds": []obj{
			{
				"op":    "<",
				"value": 1,
				"state": "down",
			},
			{
				"op":    ">=",
				"value": 2.5,
				"state": "degraded",
			},
		},
	})
	f.createPanel(id, obj{
		"type":       "stat",
		"title":      obj{"en": "Users"},
		"datasource": main,
		"query":      "5",
		"decimals":   0,
		"unit":       obj{"en": "people"},
	})
	f.createPanel(id, obj{
		"type":       "timeseries",
		"title":      obj{"en": "Load"},
		"datasource": main,
		"query":      "1 2",
		"range":      "1d",
		"step":       "5m",
		"style":      "area",
		"minZero":    true,
		"decimals":   3,
		"unit":       obj{"en": "%"},
		"legend":     obj{"en": "{{ instance }}"},
	})
	return id
}

func TestExport(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	id := f.fullSite(
		f.createDataSource("Main", "fake", fakeConfig),
		f.createDataSource("Instant", "fake-instant", fakeConfig),
	)
	w := f.admin(http.MethodGet, "/api/admin/sites/"+id+"/export", nil)
	require.Equal(http.StatusOK, w.Code)
	assert.Equal("application/yaml", w.Header().Get("Content-Type"))
	assert.Equal(
		`attachment; filename="status.shop.example.yaml"`,
		w.Header().Get("Content-Disposition"),
	)
	body := w.Body.String()
	assert.True(strings.HasPrefix(body, "version: 1\nname:\n  de: Laden\n"), body)
	assert.Contains(body, "  - type: status\n    title:\n")
	assert.Contains(body, "    datasource: Instant\n")
	assert.Contains(body, "    refresh: 1m30s\n", "durations in canonical form")
	assert.Contains(
		body,
		"  fr: Boutique\n",
		"values of languages that are not enabled",
	)
	for _, excluded := range []string{
		id,
		"availability",
		"offline",
		"createdAt",
		"revision",
		"order",
		"incidents",
	} {
		assert.NotContains(body, excluded)
	}

	missing := "/api/admin/sites/0199a000-0000-7000-8000-000000000001/export"
	assert.Equal(http.StatusNotFound, f.admin(http.MethodGet, missing, nil).Code)
}

func TestImportRoundTrip(t *testing.T) {
	assert := assert.New(t)

	a := newFixture(t)
	original := a.fullSite(
		a.createDataSource("Main", "fake", fakeConfig),
		a.createDataSource("Instant", "fake-instant", fakeConfig),
	)
	exported := a.export(original)

	b := newFixture(t)
	main := b.createDataSource("Main", "fake", fakeConfig)
	instant := b.createDataSource("Instant", "fake-instant", fakeConfig)
	res := b.yamlRequest(http.MethodPost, "/api/admin/sites/import", exported)
	imported := decode[model.Site](t, res, http.StatusCreated)
	assert.Equal(exported, b.export(imported.ID))

	// The import reproduces the site, apart from IDs, timestamps,
	// availability and the revisions the server counts.
	site := func(f *fixture, id string) (model.Site, []model.Panel) {
		w := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
		s := decode[model.Site](t, w, http.StatusOK)
		w = f.admin(http.MethodGet, "/api/admin/sites/"+id+"/panels", nil)
		panels := decode[[]model.Panel](t, w, http.StatusOK)

		s.ID = ""
		s.CreatedAt = time.Time{}
		s.UpdatedAt = time.Time{}
		for i := range panels {
			panels[i].ID = ""
			panels[i].Site = ""
			panels[i].DataSource = ""
			panels[i].Revision = 0
		}
		return s, panels
	}

	want, wantPanels := site(a, original)
	got, gotPanels := site(b, imported.ID)
	assert.Equal(model.AvailabilityOffline, want.Availability)
	assert.Equal(
		model.AvailabilityOnline,
		got.Availability,
		"imported sites start online",
	)
	got.Availability = want.Availability
	assert.Equal(want, got)
	assert.Equal(wantPanels, gotPanels)

	res = b.admin(http.MethodGet, "/api/admin/sites/"+imported.ID+"/panels", nil)
	panels := decode[[]model.Panel](t, res, http.StatusOK)
	assert.Equal(
		[]string{instant, main, main},
		[]string{panels[0].DataSource, panels[1].DataSource, panels[2].DataSource},
		"data sources are found by name",
	)
	assert.Equal(
		http.StatusOK,
		b.get("status.shop.example", "/de/").Code,
		"the site is reachable",
	)
}

func TestImportReplaces(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", fakeConfig)
	id := f.fullSite(ds, f.createDataSource("Instant", "fake-instant", fakeConfig))
	w := f.admin(http.MethodGet, "/api/admin/sites/"+id+"/panels", nil)
	old := decode[[]model.Panel](t, w, http.StatusOK)
	inc := f.createIncident(id, obj{"en": "Outage"}, obj{
		"status":      "active",
		"description": obj{"en": "Down"},
	})

	doc := `version: 1
name: {en: Replaced}
languages: {enabled: [en], primary: en}
timezone: UTC
route: {mode: path, slug: replaced}
theme: inherit
legal: {imprint: {mode: none}, privacy: {mode: inherit}}
allowedOrigins: []
incidentRetentionDays: 0
panels:
  - {type: stat, title: {en: Requests}, query: "7", datasource: main}
`
	w = f.yamlRequest(http.MethodPut, "/api/admin/sites/"+id+"/import", doc)
	site := decode[model.Site](t, w, http.StatusOK)
	assert.Equal(id, site.ID)

	w = f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	stored := decode[model.Site](t, w, http.StatusOK)
	assert.Equal(model.Text{"en": "Replaced"}, stored.Name)
	assert.Empty(stored.Logo)
	assert.Equal(
		model.AvailabilityOffline,
		stored.Availability,
		"the availability stays",
	)

	w = f.admin(http.MethodGet, "/api/admin/sites/"+id+"/panels", nil)
	panels := decode[[]model.Panel](t, w, http.StatusOK)
	require.Len(panels, 1)
	assert.Equal(ds, panels[0].DataSource, "names match regardless of case")
	assert.NotEqual(old[0].ID, panels[0].ID, "panels get new IDs")

	w = f.admin(http.MethodGet, "/api/admin/sites/"+id+"/panels/"+old[0].ID, nil)
	assert.Equal(http.StatusNotFound, w.Code)
	w = f.admin(http.MethodGet, "/api/admin/sites/"+id+"/incidents/"+inc.ID, nil)
	assert.Equal(http.StatusOK, w.Code, "incidents stay")
	w = f.get("status.example.com", "/replaced/")
	assert.Equal(http.StatusServiceUnavailable, w.Code, "the route changed")

	polled := func() bool {
		e, ok := f.poller.Entry(panels[0].ID)
		return ok && !e.Pending()
	}

	require.Eventually(
		polled,
		5*time.Second,
		10*time.Millisecond,
		"the new panel is polled",
	)
}

const minimalDocument = `version: 1
name: {en: Imported}
languages: {enabled: [en], primary: en}
route: {mode: path, slug: imported}
panels: []
`

func TestImportStrict(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	f.createDataSource("Main", "fake", fakeConfig)
	f.createDataSource("Instant", "fake-instant", fakeConfig)
	panel := func(p string) string {
		return strings.Replace(minimalDocument, "panels: []", "panels:\n"+p, 1)
	}

	tests := []struct {
		name string
		doc  string
		want map[string]string
	}{
		{"missing version", strings.Replace(minimalDocument, "version: 1\n", "", 1), map[string]string{"version": "required"}},
		{"unsupported version", strings.Replace(minimalDocument, "version: 1", "version: 2", 1), map[string]string{"version": "unsupported_version"}},
		{"textual version", strings.Replace(minimalDocument, "version: 1", `version: "1"`, 1), map[string]string{"version": "unsupported_version"}},
		{"site fields", strings.Replace(minimalDocument, "slug: imported", "slug: admin", 1) + "brandColor: red\n", map[string]string{"route.slug": "slug_reserved", "brandColor": "invalid_value"}},
		{"panel fields", panel("  - {type: stat, title: {de: Anfragen}, query: '1', datasource: Main}\n  - {type: pie, title: {en: x}, query: '1', datasource: Main}\n"), map[string]string{"panels[0].title.en": "required", "panels[1].type": "invalid_value", "panels[1].datasource": "datasource_unsupported_type"}},
		{"unknown data source", panel("  - {type: stat, title: {en: x}, query: '1', datasource: Missing}\n"), map[string]string{"panels[0].datasource": "not_found"}},
		{"no data source", panel("  - {type: stat, title: {en: x}, query: '1'}\n"), map[string]string{"panels[0].datasource": "required"}},
		{"unsupported panel type", panel("  - {type: timeseries, title: {en: x}, query: '1', range: 1h, datasource: Instant}\n"), map[string]string{"panels[0].datasource": "datasource_unsupported_type"}},
	}
	for _, tt := range tests {
		w := f.yamlRequest(http.MethodPost, "/api/admin/sites/import", tt.doc)
		assert.Equal(tt.want, fieldCodes(t, w, http.StatusBadRequest), tt.name)
	}

	for name, tt := range map[string]struct {
		doc  string
		line float64
	}{
		"unknown key":          {minimalDocument + "colour: red\n", 6},
		"unknown key of panel": {panel("  - type: stat\n    title: {en: x}\n    query: '1'\n    datasource: Main\n    color: red\n"), 10},
		"internal fields":      {minimalDocument + "id: 0199a000-0000-7000-8000-000000000001\n", 6},
		"syntax":               {"version: 1\nname: [\n", 2},
		"not a mapping":        {"- version: 1\n", 1},
	} {
		w := f.yamlRequest(http.MethodPost, "/api/admin/sites/import", tt.doc)
		body := decode[errorBody](t, w, http.StatusBadRequest)
		assert.Equal("invalid_yaml", body.Error.Code, name)
		assert.Equal(map[string]any{"line": tt.line}, body.Error.Details, name)
	}

	doc := minimalDocument + "---\nversion: 2\nunknown: [\n"
	w := f.yamlRequest(http.MethodPost, "/api/admin/sites/import", doc)
	assert.Equal(
		http.StatusCreated,
		w.Code,
		"only the first document is read: %s",
		w.Body.String(),
	)

	list := f.admin(http.MethodGet, "/api/admin/sites", nil)
	sites := len(decode[[]siteSummary](t, list, http.StatusOK))
	assert.Equal(4, sites, "failed imports write nothing")
}

func TestImportLimits(t *testing.T) {
	f := newFixture(t)
	big := minimalDocument + "logo: '" + strings.Repeat("x", 1<<20) + "'\n"
	res := f.yamlRequest(http.MethodPost, "/api/admin/sites/import", big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)

	w := f.admin(http.MethodPost, "/api/admin/sites/import", minimalDocument)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code, "imports must be YAML")

	conflict := strings.Replace(minimalDocument, "slug: imported", "slug: acme", 1)
	res = f.yamlRequest(http.MethodPost, "/api/admin/sites/import", conflict)
	want := map[string]string{"route.slug": "route_conflict"}
	assert.Equal(t, want, fieldCodes(t, res, http.StatusConflict))
}

func TestImportAtomic(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", fakeConfig)
	id := f.createSite("shop", "en")
	f.createPanel(id, obj{
		"type":       "stat",
		"title":      obj{"en": "Users"},
		"datasource": ds,
		"query":      "5",
	})
	before := f.export(id)

	invalid := strings.Replace(before, "slug: shop", "slug: changed", 1) +
		"  - {type: stat, title: {en: B}, query: '1', datasource: Gone}\n"
	w := f.yamlRequest(http.MethodPut, "/api/admin/sites/"+id+"/import", invalid)
	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Equal(before, f.export(id), "nothing is written")

	conflict := strings.Replace(before, "slug: shop", "slug: acme", 1)
	w = f.yamlRequest(http.MethodPut, "/api/admin/sites/"+id+"/import", conflict)
	assert.Equal(http.StatusConflict, w.Code)
	assert.Equal(before, f.export(id), "nothing is written")
}

package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/model"
)

func TestUnavailableSite(t *testing.T) {
	availabilities := []string{model.AvailabilityOffline, model.AvailabilityPaused}
	for _, availability := range availabilities {
		t.Run(availability, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			f := newFixture(t)
			f.putLegal([]string{"en"}, instanceLegal, nil)
			id := f.createSite("shop", "en")
			inc := f.createIncident(id, obj{"en": "Outage"}, obj{
				"status":      "active",
				"description": obj{"en": "Down"},
			})
			get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
			site := decode[model.Site](t, get, http.StatusOK)
			site.Availability = availability
			site.BrandColor = "#112233"
			site.Logo = testLogo
			site.AllowedOrigins = []string{"https://www.example.com"}
			put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
			require.Equal(http.StatusOK, put.Code)

			for _, path := range []string{
				"/shop/",
				"/shop/incidents",
				"/shop/incidents/" + inc.ID,
				"/shop/nope",
			} {
				w := f.get("status.example.com", path)
				assert.Equal(http.StatusServiceUnavailable, w.Code, path)
				assert.Contains(w.Body.String(), "<title>Site en</title>", path)
				assert.NotContains(w.Body.String(), "application/atom+xml", path)
				assert.Equal("site", parseBootstrap(t, w.Body.String()).Mode)
			}

			origin := "https://www.example.com"
			for _, path := range []string{
				"/shop/feed.atom",
				"/shop/incidents.json",
			} {
				w := f.get("status.example.com", path, "Origin", origin)
				assert.Equal(http.StatusServiceUnavailable, w.Code, path)
				assert.Empty(w.Header().Get("Access-Control-Allow-Origin"), path)
				assert.Empty(w.Header().Get("Cache-Control"), path)
			}

			w := f.get("status.example.com", "/shop/imprint")
			assert.Equal(http.StatusOK, w.Code, "legal pages still work")
			assert.NotContains(w.Body.String(), "application/atom+xml")
			privacy := f.get("status.example.com", "/shop/privacy")
			assert.Equal(http.StatusFound, privacy.Code)

			res := f.get("status.example.com", "/api/public/sites/"+id+"?lang=en")
			body := decode[struct {
				Error struct {
					Code    string     `json:"code"`
					Details siteBasics `json:"details"`
				} `json:"error"`
			}](t, res, http.StatusServiceUnavailable)
			assert.Equal("site_unavailable", body.Error.Code)
			want := siteBasics{
				Name:       "Site en",
				Theme:      "system",
				BrandColor: "#112233",
				Logo:       logoURL(&site),
				Legal: legalLinks{
					Imprint: &legalLink{Mode: "text"},
					Privacy: &legalLink{
						Mode: "url",
						URL:  "https://example.com/privacy",
					},
				},
				Languages: []string{"en"},
			}
			assert.Equal(
				want,
				body.Error.Details,
				"only what visitors see of an unavailable site",
			)

			for _, path := range []string{"/incidents", "/incidents/" + inc.ID} {
				w := f.get("status.example.com", "/api/public/sites/"+id+path)
				assert.Equal(http.StatusServiceUnavailable, w.Code, path)
				assert.Empty(w.Header().Get("Cache-Control"), path)
			}

			logo := logoURL(&site)
			assert.Equal(http.StatusOK, f.get("status.example.com", logo).Code)
			imprint := "/api/public/sites/" + id + "/legal/imprint"
			assert.Equal(http.StatusOK, f.get("status.example.com", imprint).Code)
			preview := f.admin(
				http.MethodGet,
				"/api/admin/sites/"+id+"/preview",
				nil,
			)
			assert.Equal(http.StatusOK, preview.Code, "the console previews it")
		})
	}
}

func TestPausedStatus(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	ds := f.createDataSource("Main", "fake", map[string]string{"endpoint": "x"})
	id := f.createSite("shop", "en")
	f.createPanel(id, obj{
		"type":       "status",
		"title":      obj{"en": "API"},
		"datasource": ds,
		"query":      "0",
		"refresh":    "1d",
	})
	get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](t, get, http.StatusOK)
	site.Availability = model.AvailabilityPaused
	put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	require.Equal(http.StatusOK, put.Code)

	summary := func() siteSummary {
		w := f.admin(http.MethodGet, "/api/admin/sites", nil)
		for _, sum := range decode[[]siteSummary](t, w, http.StatusOK) {
			if sum.ID == id {
				return sum
			}
		}
		t.Fatal("site missing")
		return siteSummary{}
	}

	sum := summary()
	assert.Equal(model.AvailabilityPaused, sum.Availability)
	want := siteStatus{
		Overall:   "operational",
		Panels:    "unknown",
		Incidents: "operational",
	}
	assert.Equal(
		want,
		sum.Status,
		"panels of paused sites are not polled and do not count",
	)

	f.createIncident(id, obj{"en": "Outage"}, obj{
		"status":      "active",
		"severity":    "minor",
		"description": obj{"en": "Down"},
	})
	want = siteStatus{
		Overall:   "degraded",
		Panels:    "unknown",
		Incidents: "degraded",
	}
	assert.Equal(want, summary().Status)
}

package server

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/model"
)

// createIncident creates an incident of site with an opening update and
// returns it.
func (f *fixture) createIncident(site string, title obj, update obj) incidentView {
	f.t.Helper()
	w := f.admin(http.MethodPost, "/api/admin/sites/"+site+"/incidents", obj{
		"title":  title,
		"update": update,
	})
	return decode[incidentView](f.t, w, http.StatusCreated)
}

// createFinished creates an incident of site that was resolved ago.
func (f *fixture) createFinished(site string, title obj, ago time.Duration) {
	f.t.Helper()
	inc := f.createIncident(site, title, obj{
		"status":      "active",
		"at":          at(-ago - time.Hour),
		"description": obj{"en": "x"},
	})
	path := "/api/admin/sites/" + site + "/incidents/" + inc.ID + "/updates"
	f.admin(http.MethodPost, path, obj{
		"status":      "resolved",
		"at":          at(-ago),
		"description": obj{"en": "x"},
	})
}

// at formats a time relative to now for API requests.
func at(d time.Duration) string {
	return time.Now().Add(d).UTC().Format(time.RFC3339)
}

func TestIncidentAPI(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en", "de")
	base := "/api/admin/sites/" + site + "/incidents"

	w := f.admin(http.MethodPost, base, obj{
		"title":  obj{"de": "Ausfall"},
		"update": obj{},
	})
	want := map[string]string{
		"title.en":      "required",
		"update.status": "status_or_severity_required",
	}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, base, obj{
		"title":  obj{"en": "Outage"},
		"update": obj{"status": "active"},
	})
	want = map[string]string{"update.description.en": "required"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, base, obj{
		"title": obj{"en": "Outage"},
		"update": obj{
			"status":      "resolved",
			"description": obj{"en": "x"},
		},
	})
	body := decode[errorBody](t, w, http.StatusBadRequest)
	assert.Equal("first_update_must_open", body.Error.Code)

	w = f.admin(http.MethodPost, "/api/admin/sites/missing/incidents", obj{})
	assert.Equal(http.StatusNotFound, w.Code)

	before := time.Now().Add(-time.Second)
	inc := f.createIncident(site, obj{"en": " Outage "}, obj{
		"status":      "active",
		"severity":    "major",
		"description": obj{"en": "**Down**"},
	})
	assert.Equal(model.Text{"en": "Outage"}, inc.Title)
	ann := model.Person{
		Subject:     "ann",
		DisplayName: "Ann",
	}
	assert.Equal(ann, inc.Author)
	require.Len(inc.Updates, 1)
	opening := inc.Updates[0]
	assert.Equal(ann, opening.Author)
	assert.True(opening.At.After(before), "a missing time means now")
	assert.Equal(model.Text{"en": "<p><strong>Down</strong></p>\n"}, opening.HTML)
	assert.Equal("active", inc.Status)
	assert.Equal("major", inc.Severity)
	assert.Equal("ongoing", inc.Phase)

	path := base + "/" + inc.ID
	w = f.admin(http.MethodPost, path+"/updates", obj{
		"at":       opening.At.Add(-time.Hour),
		"severity": "minor",
	})
	body = decode[errorBody](t, w, http.StatusBadRequest)
	assert.Equal(
		"first_update_must_open",
		body.Error.Code,
		"backdating before the opening update",
	)

	w = f.admin(http.MethodPost, path+"/updates", obj{})
	want = map[string]string{"status": "status_or_severity_required"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodPost, path+"/updates", obj{
		"status":      "resolved",
		"description": obj{"en": "Fixed"},
	})
	inc = decode[incidentView](t, w, http.StatusOK)
	require.Len(inc.Updates, 2)
	assert.Equal("finished", inc.Phase)
	assert.Equal("major", inc.Severity, "a status-only update keeps the severity")
	resolved := inc.Updates[1]

	w = f.admin(http.MethodPut, path+"/updates/"+resolved.ID, obj{
		"status":      "monitoring",
		"description": obj{"en": "Watching"},
	})
	edited := decode[incidentView](t, w, http.StatusOK)
	assert.Equal(
		resolved.At,
		edited.Updates[1].At,
		"a missing time leaves it unchanged",
	)
	assert.Equal("monitoring", edited.Updates[1].Status)
	assert.Equal(&ann, edited.Updates[1].EditedBy)
	assert.NotNil(edited.Updates[1].EditedAt)
	assert.Equal(resolved.Author, edited.Updates[1].Author)

	w = f.admin(http.MethodPut, path+"/updates/missing", obj{"severity": "minor"})
	assert.Equal(http.StatusNotFound, w.Code)

	w = f.admin(http.MethodPut, path, obj{"title": obj{"en": "Database outage"}})
	inc = decode[incidentView](t, w, http.StatusOK)
	assert.Equal(model.Text{"en": "Database outage"}, inc.Title)

	w = f.admin(http.MethodPut, path, obj{"title": obj{}})
	want = map[string]string{"title.en": "required"}
	assert.Equal(want, fieldCodes(t, w, http.StatusBadRequest))

	w = f.admin(http.MethodDelete, path+"/updates/"+opening.ID, nil)
	body = decode[errorBody](t, w, http.StatusBadRequest)
	assert.Equal(
		"first_update_must_open",
		body.Error.Code,
		"the next update does not open the incident",
	)

	w = f.admin(http.MethodDelete, path+"/updates/"+resolved.ID, nil)
	require.Equal(http.StatusOK, w.Code)
	w = f.admin(http.MethodDelete, path+"/updates/"+opening.ID, nil)
	require.Equal(
		http.StatusNoContent,
		w.Code,
		"deleting the last update deletes the incident",
	)
	assert.Equal(http.StatusNotFound, f.admin(http.MethodGet, path, nil).Code)

	inc = f.createIncident(site, obj{"en": "Again"}, obj{
		"severity":    "minor",
		"status":      "planned",
		"description": obj{"en": "Soon"},
	})
	w = f.admin(http.MethodDelete, base+"/"+inc.ID, nil)
	assert.Equal(http.StatusNoContent, w.Code)
	w = f.admin(http.MethodDelete, base+"/"+inc.ID, nil)
	assert.Equal(http.StatusNotFound, w.Code)
}

func TestIncidentList(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en")
	var ids []string
	for i := range adminPageSize + 1 {
		inc := f.createIncident(site, obj{"en": fmt.Sprint(i)}, obj{
			"status":      "active",
			"at":          at(time.Duration(i) * time.Minute),
			"description": obj{"en": "x"},
		})
		ids = append(ids, inc.ID)
	}

	type list struct {
		Incidents []incidentView `json:"incidents"`
		Pages     int            `json:"pages"`
	}
	w := f.admin(http.MethodGet, "/api/admin/sites/"+site+"/incidents", nil)
	l := decode[list](t, w, http.StatusOK)
	assert.Equal(2, l.Pages)
	require.Len(l.Incidents, adminPageSize)
	assert.Equal(
		ids[adminPageSize],
		l.Incidents[0].ID,
		"most recent activity first",
	)

	w = f.admin(http.MethodGet, "/api/admin/sites/"+site+"/incidents?page=2", nil)
	l = decode[list](t, w, http.StatusOK)
	assert.Equal([]string{ids[0]}, []string{l.Incidents[0].ID})

	for _, page := range []string{"0", "3", "x"} {
		path := "/api/admin/sites/" + site + "/incidents?page=" + page
		assert.Equal(
			http.StatusNotFound,
			f.admin(http.MethodGet, path, nil).Code,
			page,
		)
	}

	w = f.admin(http.MethodGet, "/api/admin/sites/missing/incidents", nil)
	assert.Equal(http.StatusNotFound, w.Code)
}

func TestMarkdownPreview(t *testing.T) {
	f := newFixture(t)
	body := obj{"text": "*a* <script>x</script>"}
	w := f.admin(http.MethodPost, "/api/admin/markdown", body)
	res := decode[map[string]string](t, w, http.StatusOK)
	assert.Equal(t, "<p><em>a</em> x</p>\n", res["html"])
}

func TestIncidentStatus(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en", "de")
	summary := func() siteSummary {
		w := f.admin(http.MethodGet, "/api/admin/sites", nil)
		for _, s := range decode[[]siteSummary](t, w, http.StatusOK) {
			if s.ID == site {
				return s
			}
		}
		t.Fatal("site not listed")
		return siteSummary{}
	}

	title := obj{
		"en": "Planned",
		"de": "Geplant",
	}
	f.createIncident(site, title, obj{
		"status":   "planned",
		"severity": "critical",
		"description": obj{
			"en": "x",
			"de": "x",
		},
	})
	want := siteStatus{
		Overall:   "operational",
		Panels:    "operational",
		Incidents: "operational",
	}
	assert.Equal(want, summary().Status, "upcoming incidents have no influence")
	assert.Equal(0, summary().Missing)

	title = obj{
		"en": "Slow",
		"de": "Langsam",
	}
	inc := f.createIncident(site, title, obj{
		"status":      "active",
		"description": obj{"en": "x"},
	})
	want = siteStatus{
		Overall:   "degraded",
		Panels:    "operational",
		Incidents: "degraded",
	}
	assert.Equal(want, summary().Status)
	assert.Equal(1, summary().Missing, "the update's description lacks German")
	p, _ := f.publicPayload(site, "")
	assert.Equal("degraded", p.Status)

	path := "/api/admin/sites/" + site + "/incidents/" + inc.ID + "/updates"
	f.admin(http.MethodPost, path, obj{"severity": "critical"})
	assert.Equal(
		"down",
		summary().Status.Overall,
		"a critical ongoing incident means down",
	)

	f.admin(http.MethodPost, path, obj{
		"status": "resolved",
		"description": obj{
			"en": "ok",
			"de": "ok",
		},
	})
	assert.Equal(
		"operational",
		summary().Status.Overall,
		"finished incidents have no influence",
	)
}

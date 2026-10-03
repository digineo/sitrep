package server

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
)

func TestIncidentsSection(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	langs := model.Languages{
		Enabled: []string{"en"},
		Primary: "en",
	}
	incident := func(id string, updates ...model.Update) model.Incident {
		return model.Incident{
			ID:      id,
			Title:   model.Text{"en": id},
			Updates: updates,
		}
	}

	up := func(d time.Duration, status, severity string) model.Update {
		return model.Update{
			At:       now.Add(d),
			Status:   status,
			Severity: severity,
		}
	}

	day := 24 * time.Hour
	incidents := []model.Incident{
		incident("ongoing", up(-2*time.Hour, "active", "critical")),
		incident("upcoming", up(-time.Hour, "planned", "")),
		incident("recent", up(-3*day, "active", "minor"), up(-2*day, "resolved", "")),
		incident("old", up(-20*day, "active", "major"), up(-10*day, "resolved", "")),
		incident("ancient", up(-40*day, "active", ""), up(-31*day, "resolved", "")),
	}
	chart := model.Panel{
		Type:  model.PanelTimeseries,
		Range: "30d",
	}

	sec, next := incidentsSection(incidents, []model.Panel{chart}, "en", langs, now)
	ids := func(list []publicIncident) []string {
		var out []string
		for _, p := range list {
			out = append(out, p.ID)
		}
		return out
	}

	assert.Equal([]string{"ongoing"}, ids(sec.Ongoing))
	assert.Equal([]string{"upcoming"}, ids(sec.Upcoming))
	assert.Equal(
		[]string{"recent"},
		ids(sec.Finished),
		"finished incidents are public for 7 days",
	)
	require.Len(
		sec.Spans,
		3,
		"spans within the widest chart range, of public incidents and others",
	)
	want := span{
		ID:       "ongoing",
		Title:    "ongoing",
		Severity: "critical",
		From:     now.Add(-2 * time.Hour),
	}
	assert.Equal(want, sec.Spans[0])
	until := now.Add(-10 * day)
	want = span{
		ID:       "old",
		Title:    "old",
		Severity: "major",
		From:     now.Add(-20 * day),
		Until:    &until,
	}
	assert.Equal(want, sec.Spans[2])
	assert.Equal(
		now.Add(5*day),
		next,
		"the recent incident stops being public first",
	)

	sec, next = incidentsSection(incidents, nil, "en", langs, now)
	assert.Empty(sec.Spans, "no charts, no spans")
	assert.Equal(now.Add(5*day), next)

	panels := []model.Panel{{
		Type:  model.PanelTimeseries,
		Range: "12d",
	}}
	sec, next = incidentsSection(incidents[3:4], panels, "en", langs, now)
	assert.Len(sec.Spans, 1)
	assert.Equal(now.Add(2*day), next, "the old span leaves the chart range")
}

func TestIncidentsCursor(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en")
	full, raw := f.publicPayload(site, "")
	require.Contains(raw, "incidents")
	assert.Empty(full.Incidents.Ongoing)

	_, raw = f.publicPayload(site, "?since="+full.Cursor)
	assert.Equal([]string{"cursor", "status"}, keys(raw))

	inc := f.createIncident(site, obj{"en": "Outage"}, obj{
		"status":      "active",
		"description": obj{"en": "Down"},
	})
	p, raw := f.publicPayload(site, "?since="+full.Cursor)
	assert.Equal(
		[]string{"cursor", "incidents", "status"},
		keys(raw),
		"an incident change returns the incidents section",
	)
	assert.Equal("degraded", p.Status)
	require.Len(p.Incidents.Ongoing, 1)
	assert.Equal(inc.ID, p.Incidents.Ongoing[0].ID)
	assert.Equal("<p>Down</p>\n", p.Incidents.Ongoing[0].Updates[0].HTML)

	f.poller.ExpireIncidents(site, time.Now().Add(-time.Second))
	_, raw = f.publicPayload(site, "?since="+p.Cursor)
	assert.Contains(raw, "incidents", "an expired view counts as a change")
}

func TestPublicIncidents(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en", "de")
	title := obj{
		"en": "Old",
		"de": "Alt",
	}
	old := f.createIncident(site, title, obj{
		"status":      "active",
		"at":          at(-30 * 24 * time.Hour),
		"description": obj{"en": "Was down"},
	})
	updates := "/api/admin/sites/" + site + "/incidents/" + old.ID + "/updates"
	f.admin(http.MethodPost, updates, obj{
		"status":      "resolved",
		"at":          at(-29 * 24 * time.Hour),
		"description": obj{"en": "Fixed"},
	})

	w := f.get(
		"status.example.com",
		"/api/public/sites/"+site+"/incidents/"+old.ID+"?lang=de",
	)
	require.Equal(http.StatusOK, w.Code)
	assert.Equal("public, max-age=10", w.Header().Get("Cache-Control"))
	var detail publicIncident
	require.NoError(json.Unmarshal(w.Body.Bytes(), &detail))
	assert.Equal(
		"Alt",
		detail.Title,
		"every incident has a detail page, public or not",
	)
	assert.Equal("finished", detail.Phase)
	assert.Len(detail.Updates, 2)
	assert.NotContains(w.Body.String(), "Ann", "authors are not public")

	invalid := "/api/public/sites/" + site + "/incidents/x"
	assert.Equal(http.StatusBadRequest, f.get("status.example.com", invalid).Code)
	missing := "/api/public/sites/" + site + "/incidents/" + uuid.NewV7().String()
	assert.Equal(http.StatusNotFound, f.get("status.example.com", missing).Code)

	for i := range archivePageSize {
		f.createIncident(site, obj{"en": fmt.Sprint(i)}, obj{
			"status":      "planned",
			"description": obj{"en": "x"},
		})
	}

	var archive struct {
		Incidents []publicIncident `json:"incidents"`
		Pages     int              `json:"pages"`
	}
	w = f.get("status.example.com", "/api/public/sites/"+site+"/incidents?page=2")
	require.Equal(http.StatusOK, w.Code)
	require.NoError(json.Unmarshal(w.Body.Bytes(), &archive))
	assert.Equal(2, archive.Pages)
	assert.Equal(
		[]string{old.ID},
		[]string{archive.Incidents[0].ID},
		"the archive lists all incidents",
	)
	beyond := "/api/public/sites/" + site + "/incidents?page=3"
	assert.Equal(http.StatusNotFound, f.get("status.example.com", beyond).Code)

	w = f.get("status.example.com", "/shop/de/incidents/"+old.ID)
	assert.Equal(http.StatusOK, w.Code)
	assert.Contains(w.Body.String(), "<title>Alt · Site de</title>")
	assert.Contains(w.Body.String(), `<link rel="alternate" type="application/atom+xml" href="http://status.example.com/shop/de/feed.atom">`)
	for _, path := range []string{
		"/shop/de/incidents/" + uuid.NewV7().String(),
		"/shop/de/incidents/x",
		"/shop/de/incidents?page=3",
		"/shop/de/incidents?page=0",
	} {
		assert.Equal(
			http.StatusNotFound,
			f.get("status.example.com", path).Code,
			path,
		)
	}

	assert.Equal(
		http.StatusOK,
		f.get("status.example.com", "/shop/de/incidents?page=2").Code,
	)
}

func TestUUIDv5(t *testing.T) {
	dns := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	got := uuidV5(dns, "www.example.com")
	assert.Equal(t, "2ed6657d-e927-568b-95e1-2665a8aea6a2", got.String())
}

func TestFormatTime(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(err)

	en, de := i18n.Get("en"), i18n.Get("de")
	summer := time.Date(2026, 7, 1, 8, 5, 0, 0, time.UTC)
	winter := time.Date(2026, 12, 24, 23, 30, 0, 0, time.UTC)
	assert.Equal("2026-07-01 10:05 +02:00", formatTime(en, summer, berlin))
	assert.Equal("01.07.2026, 10:05 +02:00", formatTime(de, summer, berlin))
	assert.Equal("2026-12-25 00:30 +01:00", formatTime(en, winter, berlin))
	assert.Equal("24.12.2026, 23:30 +00:00", formatTime(de, winter, time.UTC))
}

type testFeed struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
	Lang    string   `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Updated string   `xml:"updated"`
	Author  string   `xml:"author>name"`
	Links   []struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Entries []struct {
		ID      string `xml:"id"`
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Link    struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Content string `xml:"content"`
	} `xml:"entry"`
}

func TestFeed(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en", "de")
	title := obj{
		"en": "Outage",
		"de": "Ausfall",
	}
	inc := f.createIncident(site, title, obj{
		"status": "active",
		"at":     "2026-10-01T10:00:00Z",
		"description": obj{
			"en": "Down",
			"de": "**Gestört**",
		},
	})
	path := "/api/admin/sites/" + site + "/incidents/" + inc.ID + "/updates"
	f.admin(http.MethodPost, path, obj{
		"severity": "critical",
		"at":       at(-time.Minute),
	})
	f.createFinished(site, obj{"en": "Old"}, 20*24*time.Hour)

	w := f.get("status.example.com", "/shop/de/feed.atom")
	require.Equal(http.StatusOK, w.Code)
	assert.Equal(
		"application/atom+xml; charset=utf-8",
		w.Header().Get("Content-Type"),
	)
	assert.Equal("public, max-age=60", w.Header().Get("Cache-Control"))
	assert.True(strings.HasPrefix(w.Body.String(), xml.Header))
	assert.NotContains(w.Body.String(), "Ann", "authors are not public")
	var feed testFeed
	require.NoError(xml.Unmarshal(w.Body.Bytes(), &feed))
	assert.Equal("de", feed.Lang)
	assert.Equal("urn:uuid:"+uuidV5(uuid.MustParse(site), "de").String(), feed.ID)
	assert.NotEqual(
		feed.ID,
		"urn:uuid:"+uuidV5(uuid.MustParse(site), "en").String(),
	)
	assert.Equal("Site de", feed.Title)
	assert.Equal("Site de", feed.Author)
	assert.Equal("http://status.example.com/shop/de/feed.atom", feed.Links[0].Href)
	assert.Equal("http://status.example.com/shop/de/", feed.Links[1].Href)

	require.Len(feed.Entries, 2, "one entry per update of public incidents")
	latest, opening := feed.Entries[0], feed.Entries[1]
	assert.Equal("Ausfall: Schweregrad: Kritisch", latest.Title, "newest first")
	assert.Equal(latest.Updated, feed.Updated)
	assert.Equal("urn:uuid:"+inc.Updates[0].ID, opening.ID)
	assert.Equal("Ausfall: Aktiv", opening.Title)
	assert.Equal("2026-10-01T10:00:00Z", opening.Updated)
	assert.Equal(
		"http://status.example.com/shop/de/incidents/"+inc.ID,
		opening.Link.Href,
	)
	assert.Contains(opening.Content, "<p><strong>Gestört</strong></p>")
	assert.Contains(opening.Content, `<a href="http://status.example.com/shop/de/incidents/`+inc.ID+`">Vorfall ansehen</a>`)
	assert.NotContains(opening.Content, "Frühere Meldungen")
	assert.Contains(latest.Content, "Frühere Meldungen")
	assert.Contains(html.UnescapeString(latest.Content), `<time datetime="2026-10-01T10:00:00Z">01.10.2026, 10:00 +00:00</time> Aktiv: Gestört`)

	w = f.get("status.example.com", "/shop/feed.atom", "Accept-Language", "en")
	assert.Equal(http.StatusFound, w.Code)
	assert.Equal("/shop/en/feed.atom", w.Header().Get("Location"))
}

func TestFeedWithoutEntries(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en")
	stored, err := f.db.Site(site)
	require.NoError(err)

	var feed testFeed
	w := f.get("status.example.com", "/shop/feed.atom")
	require.NoError(xml.Unmarshal(w.Body.Bytes(), &feed))
	assert.Empty(feed.Entries)
	assert.Equal(
		stored.UpdatedAt.Format(time.RFC3339Nano),
		feed.Updated,
		"the site's update time",
	)
}

func TestIncidentsJSON(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	site := f.createSite("shop", "en", "de")
	get := f.admin(http.MethodGet, "/api/admin/sites/"+site, nil)
	settings := decode[model.Site](t, get, http.StatusOK)
	settings.AllowedOrigins = []string{"https://www.example.com"}
	put := f.admin(http.MethodPut, "/api/admin/sites/"+site, settings)
	require.Equal(http.StatusOK, put.Code)
	title := obj{
		"en": "Outage",
		"de": "Ausfall",
	}
	f.createIncident(site, title, obj{
		"status":      "active",
		"severity":    "minor",
		"description": obj{"en": "Down"},
	})
	f.createFinished(site, obj{"en": "Old"}, 20*24*time.Hour)

	w := f.get(
		"status.example.com",
		"/shop/de/incidents.json",
		"Origin",
		"HTTPS://www.example.com:443",
	)
	require.Equal(http.StatusOK, w.Code)
	assert.Equal(
		"HTTPS://www.example.com:443",
		w.Header().Get("Access-Control-Allow-Origin"),
		"origins match normalized",
	)
	assert.Equal("Origin", w.Header().Get("Vary"))
	assert.Equal("public, max-age=60", w.Header().Get("Cache-Control"))
	assert.Empty(w.Header().Get("Access-Control-Allow-Credentials"))
	var list []publicIncident
	require.NoError(json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(list, 1, "only public incidents")
	assert.Equal("Ausfall", list[0].Title)
	assert.Equal("ongoing", list[0].Phase)
	assert.Equal("minor", list[0].Severity)
	assert.Equal("<p>Down</p>\n", list[0].Updates[0].HTML)
	assert.Regexp(`"at":"[0-9T:-]+Z"`, w.Body.String(), "times in UTC")

	for _, origin := range []string{"https://evil.example.com", "null", ""} {
		w = f.get("status.example.com", "/shop/de/incidents.json", "Origin", origin)
		assert.Empty(w.Header().Get("Access-Control-Allow-Origin"), origin)
		assert.Equal("Origin", w.Header().Get("Vary"), origin)
	}
}

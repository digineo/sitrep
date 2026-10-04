package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

func openTemp(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sitrep.db")
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func TestOpenFresh(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, path := openTemp(t)
	fi, err := os.Stat(path)
	require.NoError(err)
	assert.Equal(os.FileMode(0o600), fi.Mode().Perm())

	s, err := db.Settings()
	require.NoError(err)
	assert.Equal(model.DefaultSettings(), s)
	assert.NoError(db.Check())
}

func TestOpenKeepsData(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, path := openTemp(t)
	s := model.Settings{
		Languages: model.Languages{
			Enabled: []string{"de"},
			Primary: "de",
		},
		DefaultTheme: "dark",
	}
	require.NoError(db.PutSettings(s))
	require.NoError(db.Close())

	db, err := Open(path)
	require.NoError(err)
	defer func() { _ = db.Close() }()

	got, err := db.Settings()
	require.NoError(err)
	assert.Equal(s, got)
}

func TestOpenLocked(t *testing.T) {
	_, path := openTemp(t)
	start := time.Now()
	_, err := Open(path)
	assert.ErrorContains(t, err, "in use by another process")
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestOpenNewerSchema(t *testing.T) {
	db, path := openTemp(t)
	require.NoError(t, db.bolt.Update(func(tx *bolt.Tx) error {
		return put(tx, bucketMeta, keySchema, schemaVersion+1)
	}))

	require.NoError(t, db.Close())

	_, err := Open(path)
	assert.ErrorContains(t, err, "newer SitRep")
}

func TestSessions(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	now := time.Now().UTC().Truncate(time.Second)
	live := model.Session{
		Provider:    "basic",
		Subject:     "ann",
		DisplayName: "Ann",
		Expires:     now.Add(time.Hour),
	}
	require.NoError(db.CreateSession([]byte("live"), live))
	expired := model.Session{Expires: now}
	require.NoError(db.CreateSession([]byte("expired"), expired))
	expired2 := model.Session{Expires: now.Add(-time.Hour)}
	require.NoError(db.CreateSession([]byte("expired2"), expired2))

	got, found, err := db.Session([]byte("live"))
	require.NoError(err)
	assert.True(found)
	assert.Equal(live, got)

	n, err := db.PurgeSessions(now)
	require.NoError(err)
	assert.Equal(2, n)
	_, found, _ = db.Session([]byte("expired"))
	assert.False(found)
	_, found, _ = db.Session([]byte("live"))
	assert.True(found)

	require.NoError(db.DeleteSession([]byte("live")))
	_, found, _ = db.Session([]byte("live"))
	assert.False(found)
}

func TestSiteRoutes(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	path := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "acme",
	}}
	require.NoError(db.CreateSite(path))
	assert.NotEmpty(path.ID)

	sub := &model.Site{Route: model.Route{
		Mode: model.RouteSubdomain,
		Slug: "acme",
	}}
	require.NoError(
		db.CreateSite(sub),
		"path and subdomain slugs are separate namespaces",
	)
	require.NoError(db.CreateSite(&model.Site{Route: model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.acme.com",
	}}))

	err := db.CreateSite(&model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "acme",
	}})
	var e *apierr.Error
	require.True(errors.As(err, &e))
	assert.Equal(409, e.Status)
	assert.Equal(apierr.RouteConflict, e.Code)

	got, err := db.SiteByRoute(model.Route{
		Mode: model.RouteSubdomain,
		Slug: "acme",
	})
	require.NoError(err)
	assert.Equal(sub, got)
	got, err = db.SiteByRoute(model.Route{
		Mode:   model.RouteCustom,
		Domain: "acme",
	})
	require.NoError(err)
	assert.Nil(got)

	sites, err := db.Sites()
	require.NoError(err)
	assert.Len(sites, 3)
}

func apiError(t *testing.T, err error) *apierr.Error {
	t.Helper()
	e, ok := errors.AsType[*apierr.Error](err)
	require.True(t, ok, err)
	return e
}

func TestUpdateAndDeleteSite(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	a := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	b := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "b",
	}}
	require.NoError(db.CreateSite(a))
	require.NoError(db.CreateSite(b))

	moved := *a
	moved.Route = model.Route{
		Mode:   model.RouteCustom,
		Domain: "status.a.com",
	}
	moved.CreatedAt = time.Time{}
	require.NoError(db.UpdateSite(&moved))
	assert.Equal(a.CreatedAt, moved.CreatedAt, "the creation time is kept")
	got, err := db.SiteByRoute(model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	})
	require.NoError(err)
	assert.Nil(got, "the old route is free")
	require.NoError(db.CreateSite(&model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}))

	moved.Route = b.Route
	e := apiError(t, db.UpdateSite(&moved))
	assert.Equal(409, e.Status)
	want := []apierr.Field{{
		Path: "route.slug",
		Code: "route_conflict",
	}}
	assert.Equal(want, e.Fields)

	ds := &model.DataSource{
		ID:   "ds",
		Name: "x",
		Type: "prometheus",
	}
	require.NoError(db.CreateDataSource(ds))
	require.NoError(db.CreatePanel(&model.Panel{
		Site:       b.ID,
		DataSource: "ds",
	}))
	require.NoError(db.CreateIncident(&model.Incident{
		Site:    b.ID,
		Updates: []model.Update{{Status: model.StatusActive}},
	}))
	require.NoError(db.DeleteSite(b.ID))
	_, err = db.Site(b.ID)
	assert.Equal(404, apiError(t, err).Status)
	snap, err := db.Snapshot()
	require.NoError(err)
	assert.Empty(snap.Panels, "panels go with their site")
	assert.Empty(snap.Incidents, "incidents go with their site")
	got, err = db.SiteByRoute(b.Route)
	require.NoError(err)
	assert.Nil(got)
	assert.Equal(404, apiError(t, db.DeleteSite(b.ID)).Status)
}

func TestLandingSite(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	settings := model.DefaultSettings()
	settings.LandingSite = "missing"
	e := apiError(t, db.PutSettings(settings))
	assert.Equal(400, e.Status)
	want := []apierr.Field{{
		Path: "landingSite",
		Code: "not_found",
	}}
	assert.Equal(want, e.Fields)

	a := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	b := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "b",
	}}
	require.NoError(db.CreateSite(a))
	require.NoError(db.CreateSite(b))
	settings.LandingSite = a.ID
	require.NoError(db.PutSettings(settings))

	require.NoError(db.DeleteSite(b.ID))
	got, err := db.Settings()
	require.NoError(err)
	assert.Equal(a.ID, got.LandingSite)

	require.NoError(db.DeleteSite(a.ID))
	got, err = db.Settings()
	require.NoError(err)
	assert.Empty(got.LandingSite, "a deleted site is no longer promoted")
}

func TestPanels(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	site := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	require.NoError(db.CreateSite(site))
	require.NoError(db.CreateDataSource(&model.DataSource{
		ID:   "ds",
		Name: "x",
	}))

	e := apiError(t, db.CreatePanel(&model.Panel{
		Site:       site.ID,
		DataSource: "missing",
	}))
	want := []apierr.Field{{
		Path: "datasource",
		Code: "not_found",
	}}
	assert.Equal(want, e.Fields)
	orphan := &model.Panel{
		Site:       "missing",
		DataSource: "ds",
	}
	assert.Equal(404, apiError(t, db.CreatePanel(orphan)).Status)

	var ids []string
	for range 3 {
		p := &model.Panel{
			Site:       site.ID,
			DataSource: "ds",
			Order:      7,
			Revision:   7,
		}
		require.NoError(db.CreatePanel(p))
		assert.Equal(int64(1), p.Revision)
		ids = append(ids, p.ID)
	}

	panels, err := db.Panels(site.ID)
	require.NoError(err)
	orders := []int{panels[0].Order, panels[1].Order, panels[2].Order}
	assert.Equal([]int{0, 1, 2}, orders, "new panels go last")

	p := panels[1]
	p.Order = 9
	p.Revision = 9
	p.Query = "up"
	require.NoError(db.UpdatePanel(&p))
	assert.Equal(1, p.Order, "the order is server-managed")
	assert.Equal(int64(2), p.Revision)

	require.NoError(db.ReorderPanels(site.ID, []string{ids[2], ids[0], ids[1]}))
	panels, err = db.Panels(site.ID)
	require.NoError(err)
	reordered := []string{panels[0].ID, panels[1].ID, panels[2].ID}
	assert.Equal([]string{ids[2], ids[0], ids[1]}, reordered)
	assert.Equal(int64(2), panels[2].Revision, "reordering keeps revisions")
	for _, order := range [][]string{
		{ids[0], ids[1]},
		{ids[0], ids[1], ids[1]},
		{ids[0], ids[1], "x"},
	} {
		err := db.ReorderPanels(site.ID, order)
		assert.Equal(400, apiError(t, err).Status, order)
	}

	require.NoError(db.DeletePanel(site.ID, ids[0]))
	_, err = db.Panel(site.ID, ids[0])
	assert.Equal(404, apiError(t, err).Status)
	p4 := &model.Panel{
		Site:       site.ID,
		DataSource: "ds",
	}
	require.NoError(db.CreatePanel(p4))
	assert.Equal(3, p4.Order)
}

func TestDataSources(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	a := &model.DataSource{
		ID:   "a",
		Name: "Main",
		Type: "prometheus",
	}
	require.NoError(db.CreateDataSource(a))
	assert.Equal(int64(1), a.Revision)
	e := apiError(t, db.CreateDataSource(&model.DataSource{
		ID:   "b",
		Name: "MAIN",
	}))
	assert.Equal(409, e.Status)
	want := []apierr.Field{{
		Path: "name",
		Code: "name_taken",
	}}
	assert.Equal(want, e.Fields)

	b := &model.DataSource{
		ID:   "b",
		Name: "Other",
		Type: "prometheus",
	}
	require.NoError(db.CreateDataSource(b))
	b.Name = "main"
	assert.Equal(409, apiError(t, db.UpdateDataSource(b)).Status)
	a.Name = "Main"
	a.Type = "changed"
	require.NoError(db.UpdateDataSource(a))
	assert.Equal(int64(2), a.Revision)
	assert.Equal("prometheus", a.Type, "the type is immutable")

	s1 := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	s2 := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "b",
	}}
	require.NoError(db.CreateSite(s1))
	require.NoError(db.CreateSite(s2))
	for _, site := range []string{s1.ID, s1.ID, s2.ID} {
		require.NoError(db.CreatePanel(&model.Panel{
			Site:       site,
			DataSource: "a",
		}))
	}

	e = apiError(t, db.DeleteDataSource("a"))
	assert.Equal(409, e.Status)
	assert.Equal("datasource_in_use", e.Code)
	inUse := InUse{
		Panels: 3,
		Sites:  []string{s1.ID, s2.ID},
	}
	assert.Equal(inUse, e.Details)

	require.NoError(db.DeleteDataSource("b"))
	all, err := db.DataSources()
	require.NoError(err)
	require.Len(all, 1)
	assert.Equal("a", all[0].ID)
}

func TestIncidents(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	site := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	require.NoError(db.CreateSite(site))
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	resolved := &model.Incident{
		Site: site.ID,
		Updates: []model.Update{{
			At:     t0,
			Status: model.StatusResolved,
		}},
	}
	assert.Equal(400, apiError(t, db.CreateIncident(resolved)).Status)
	orphan := &model.Incident{
		Site: "missing",
		Updates: []model.Update{{
			At:     t0,
			Status: model.StatusActive,
		}},
	}
	assert.Equal(404, apiError(t, db.CreateIncident(orphan)).Status)

	inc := &model.Incident{
		Site: site.ID,
		Updates: []model.Update{{
			ID:     "u1",
			At:     t0,
			Status: model.StatusActive,
		}},
	}
	require.NoError(db.CreateIncident(inc))
	require.NotEmpty(inc.ID)
	assert.False(inc.CreatedAt.IsZero())

	backdate := func(i *model.Incident) error {
		i.Updates = append(i.Updates, model.Update{
			ID:       "u0",
			At:       t0.Add(-time.Hour),
			Severity: model.SeverityMajor,
		})
		return nil
	}

	_, err := db.ChangeIncident(site.ID, inc.ID, backdate)
	assert.Equal("first_update_must_open", apiError(t, err).Code)
	got, err := db.Incident(site.ID, inc.ID)
	require.NoError(err)
	assert.Len(got.Updates, 1, "a rejected change is not stored")

	addPlanned := func(i *model.Incident) error {
		i.Updates = append(i.Updates, model.Update{
			ID:     "u2",
			At:     t0.Add(-time.Hour),
			Status: model.StatusPlanned,
		})
		return nil
	}

	changed, err := db.ChangeIncident(site.ID, inc.ID, addPlanned)
	require.NoError(err)
	ids := []string{changed.Updates[0].ID, changed.Updates[1].ID}
	assert.Equal([]string{"u2", "u1"}, ids, "updates are sorted by time")

	all, err := db.Incidents(site.ID)
	require.NoError(err)
	assert.Len(all, 1)
	_, err = db.Incidents("missing")
	assert.Equal(404, apiError(t, err).Status)

	gone, err := db.ChangeIncident(site.ID, inc.ID, func(i *model.Incident) error {
		i.Updates = nil
		return nil
	})
	require.NoError(err)
	assert.Nil(gone, "an incident without updates is deleted")
	_, err = db.Incident(site.ID, inc.ID)
	assert.Equal(404, apiError(t, err).Status)
	_, err = db.ChangeIncident(site.ID, inc.ID, backdate)
	assert.Equal(404, apiError(t, err).Status)

	require.NoError(db.CreateIncident(inc))
	require.NoError(db.DeleteIncident(site.ID, inc.ID))
	assert.Equal(404, apiError(t, db.DeleteIncident(site.ID, inc.ID)).Status)
}

func TestPurgeIncidents(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	kept := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "kept",
	}}
	purged := &model.Site{
		Route: model.Route{
			Mode: model.RoutePath,
			Slug: "purged",
		},
		IncidentRetentionDays: 2,
	}
	require.NoError(db.CreateSite(kept))
	require.NoError(db.CreateSite(purged))

	create := func(site string, statuses ...string) string {
		inc := &model.Incident{Site: site}
		for i, s := range statuses {
			inc.Updates = append(inc.Updates, model.Update{
				At:     t0.Add(time.Duration(i) * time.Hour),
				Status: s,
			})
		}
		require.NoError(db.CreateIncident(inc))
		return inc.ID
	}

	create(kept.ID, model.StatusActive, model.StatusResolved)
	old := create(purged.ID, model.StatusActive, model.StatusResolved)
	create(purged.ID, model.StatusPlanned)
	create(purged.ID, model.StatusActive)

	lastActivity := t0.Add(time.Hour)
	n, err := db.PurgeIncidents(lastActivity.Add(48 * time.Hour))
	require.NoError(err)
	assert.Empty(n, "exactly the retention period keeps the incident")

	n, err = db.PurgeIncidents(lastActivity.Add(48*time.Hour + time.Second))
	require.NoError(err)
	assert.Equal(map[string]int{purged.ID: 1}, n)
	_, err = db.Incident(purged.ID, old)
	assert.Equal(404, apiError(t, err).Status)
	snap, err := db.Snapshot()
	require.NoError(err)
	assert.Len(
		snap.Incidents,
		3,
		"sites without retention, upcoming and ongoing incidents keep theirs",
	)
}

func TestImportSite(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	require.NoError(db.CreateDataSource(&model.DataSource{
		ID:   "ds",
		Name: "Main",
		Type: "fake",
	}))
	site := &model.Site{
		Name: model.Text{"en": "Shop"},
		Route: model.Route{
			Mode: model.RoutePath,
			Slug: "shop",
		},
		Availability: model.AvailabilityPaused,
	}
	require.NoError(db.CreateSite(site))
	old := &model.Panel{
		Site:       site.ID,
		Type:       model.PanelStat,
		DataSource: "ds",
	}
	require.NoError(db.CreatePanel(old))
	inc := &model.Incident{
		Site:    site.ID,
		Title:   model.Text{"en": "Outage"},
		Updates: []model.Update{{Status: model.StatusActive}},
	}
	require.NoError(db.CreateIncident(inc))

	replaced := &model.Site{
		ID:   site.ID,
		Name: model.Text{"en": "Replaced"},
		Route: model.Route{
			Mode: model.RoutePath,
			Slug: "replaced",
		},
	}
	panels := []model.Panel{
		{
			Type:       model.PanelStat,
			DataSource: "ds",
		},
		{
			Type:       model.PanelStat,
			DataSource: "gone",
		},
	}
	err := db.ImportSite(replaced, panels)
	var e *apierr.Error
	require.ErrorAs(err, &e)
	want := []apierr.Field{{
		Path: "panels[1].datasource",
		Code: apierr.NotFound,
	}}
	assert.Equal(want, e.Fields)
	stored, err := db.Site(site.ID)
	require.NoError(err)
	assert.Equal(
		model.Text{"en": "Shop"},
		stored.Name,
		"a failed import writes nothing",
	)
	got, err := db.Panels(site.ID)
	require.NoError(err)
	assert.Equal([]model.Panel{*old}, got)

	require.NoError(db.ImportSite(replaced, panels[:1]))
	stored, err = db.Site(site.ID)
	require.NoError(err)
	assert.Equal(model.Text{"en": "Replaced"}, stored.Name)
	assert.Equal(
		model.AvailabilityPaused,
		stored.Availability,
		"the availability stays",
	)
	assert.Equal(site.CreatedAt, stored.CreatedAt)
	got, err = db.Panels(site.ID)
	require.NoError(err)
	require.Len(got, 1)
	assert.NotEqual(old.ID, got[0].ID)
	assert.Equal(int64(1), got[0].Revision)
	_, err = db.Incident(site.ID, inc.ID)
	assert.NoError(err, "incidents stay")
	found, err := db.SiteByRoute(model.Route{
		Mode: model.RoutePath,
		Slug: "replaced",
	})
	require.NoError(err)
	assert.Equal(site.ID, found.ID)

	created := &model.Site{
		Name: model.Text{"en": "New"},
		Route: model.Route{
			Mode: model.RoutePath,
			Slug: "replaced",
		},
	}
	err = db.ImportSite(created, nil)
	require.ErrorAs(err, &e)
	assert.Equal(apierr.RouteConflict, e.Code)
	created.Route.Slug = "new"
	require.NoError(db.ImportSite(created, panels[:1]))
	assert.NotEmpty(created.ID)
	assert.Equal(model.AvailabilityOnline, created.Availability)
}

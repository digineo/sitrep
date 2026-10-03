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
	require.NoError(db.DeleteSite(b.ID))
	_, err = db.Site(b.ID)
	assert.Equal(404, apiError(t, err).Status)
	snap, err := db.Snapshot()
	require.NoError(err)
	assert.Empty(snap.Panels, "panels go with their site")
	got, err = db.SiteByRoute(b.Route)
	require.NoError(err)
	assert.Nil(got)
	assert.Equal(404, apiError(t, db.DeleteSite(b.ID)).Status)
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

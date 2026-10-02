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

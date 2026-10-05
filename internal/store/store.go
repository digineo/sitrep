// Package store persists SitRep's data in an embedded BBolt database.
// Values are JSON-encoded, and every operation is one transaction.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// schemaVersion is the version of the database layout written by this build.
const schemaVersion = 2

var (
	bucketMeta      = []byte("meta")
	bucketSettings  = []byte("settings")
	bucketSessions  = []byte("sessions")
	bucketSites     = []byte("sites")
	bucketRoutes    = []byte("routes")
	bucketPanels    = []byte("panels")
	bucketSources   = []byte("datasources")
	bucketIncidents = []byte("incidents")
	bucketAccounts  = []byte("accounts")

	keySchema   = []byte("schema")
	keyInstance = []byte("instance")
)

// DB is the database.
type DB struct {
	bolt *bolt.DB
}

// Open opens or creates the database file. A fresh database gets the
// default settings.
func Open(path string) (*DB, error) {
	b, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 100 * time.Millisecond})
	if errors.Is(err, berrors.ErrTimeout) {
		return nil, fmt.Errorf("database %s is in use by another process", path)
	}
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	db := &DB{bolt: b}
	if err := b.Update(db.init); err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return db, nil
}

func (db *DB) init(tx *bolt.Tx) error {
	buckets := [][]byte{
		bucketMeta,
		bucketSettings,
		bucketSessions,
		bucketSites,
		bucketRoutes,
		bucketPanels,
		bucketSources,
		bucketIncidents,
		bucketAccounts,
	}
	for _, name := range buckets {
		if _, err := tx.CreateBucketIfNotExists(name); err != nil {
			return err
		}
	}

	var version int
	found, err := get(tx, bucketMeta, keySchema, &version)
	switch {
	case err != nil:
		return err
	case !found:
		err := put(tx, bucketSettings, keyInstance, model.DefaultSettings())
		if err != nil {
			return err
		}
		return put(tx, bucketMeta, keySchema, schemaVersion)
	case version > schemaVersion:
		return fmt.Errorf(
			"written by a newer SitRep (schema %d, this build supports %d)",
			version,
			schemaVersion,
		)
	case version < 2:
		if err := accountsFromSessions(tx); err != nil {
			return err
		}
		return put(tx, bucketMeta, keySchema, schemaVersion)
	}
	return nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.bolt.Close()
}

// Check reports whether the database is readable.
func (db *DB) Check() error {
	return db.bolt.View(func(tx *bolt.Tx) error {
		var version int
		_, err := get(tx, bucketMeta, keySchema, &version)
		return err
	})
}

var errNotFound = apierr.New(http.StatusNotFound, apierr.NotFound)

// mustGet reads the value under key, failing with a not-found error if
// there is none.
func mustGet(tx *bolt.Tx, bucket, key []byte, v any) error {
	found, err := get(tx, bucket, key, v)
	if err == nil && !found {
		return errNotFound
	}
	return err
}

// list decodes the values of a bucket, or of the keys with prefix.
func list[T any](tx *bolt.Tx, bucket, prefix []byte) ([]T, error) {
	var out []T
	c := tx.Bucket(bucket).Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var item T
		if err := json.Unmarshal(v, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// now returns the current time for timestamps, in UTC.
func now() time.Time {
	return time.Now().UTC()
}

func get(tx *bolt.Tx, bucket, key []byte, v any) (bool, error) {
	raw := tx.Bucket(bucket).Get(key)
	if raw == nil {
		return false, nil
	}
	return true, json.Unmarshal(raw, v)
}

func put(tx *bolt.Tx, bucket, key []byte, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return tx.Bucket(bucket).Put(key, raw)
}

// Snapshot is the state of all sites, panels, data sources and incidents
// at one point in time.
type Snapshot struct {
	Sites       []model.Site
	Panels      []model.Panel // by site, in display order
	DataSources []model.DataSource
	Incidents   []model.Incident // by site
}

// Snapshot reads all sites, panels, data sources and incidents in one
// transaction.
func (db *DB) Snapshot() (Snapshot, error) {
	var s Snapshot
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		if s.Sites, err = list[model.Site](tx, bucketSites, nil); err != nil {
			return err
		}

		if s.Panels, err = list[model.Panel](tx, bucketPanels, nil); err != nil {
			return err
		}

		s.DataSources, err = list[model.DataSource](tx, bucketSources, nil)
		if err != nil {
			return err
		}

		s.Incidents, err = list[model.Incident](tx, bucketIncidents, nil)
		return err
	})

	sortPanels(s.Panels)
	return s, err
}

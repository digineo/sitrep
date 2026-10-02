// Package store persists SitRep's data in an embedded BBolt database.
// Values are JSON-encoded, and every operation is one transaction.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"

	"github.com/digineo/sitrep/internal/model"
)

// schemaVersion is the version of the database layout written by this build.
const schemaVersion = 1

var (
	bucketMeta     = []byte("meta")
	bucketSettings = []byte("settings")
	bucketSessions = []byte("sessions")
	bucketSites    = []byte("sites")
	bucketRoutes   = []byte("routes")

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

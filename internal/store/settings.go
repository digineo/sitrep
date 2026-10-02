package store

import (
	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/model"
)

// Settings returns the instance settings.
func (db *DB) Settings() (model.Settings, error) {
	var s model.Settings
	err := db.bolt.View(func(tx *bolt.Tx) error {
		_, err := get(tx, bucketSettings, keyInstance, &s)
		return err
	})
	return s, err
}

// PutSettings replaces the instance settings.
func (db *DB) PutSettings(s model.Settings) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return put(tx, bucketSettings, keyInstance, s)
	})
}

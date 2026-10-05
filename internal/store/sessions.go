package store

import (
	"bytes"
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/model"
)

// CreateSession stores a session under the hash of its token.
func (db *DB) CreateSession(hash []byte, s model.Session) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return put(tx, bucketSessions, hash, s)
	})
}

// Session returns the session stored under hash, whether expired or not,
// and its account, or nil if the account is gone.
func (db *DB) Session(hash []byte) (model.Session, *model.Account, bool, error) {
	var s model.Session
	var acc *model.Account
	var found bool
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		found, err = get(tx, bucketSessions, hash, &s)
		if err != nil || !found {
			return err
		}

		var a model.Account
		if ok, err := get(tx, bucketAccounts, []byte(s.Account), &a); err != nil || !ok {
			return err
		}
		acc = &a
		return nil
	})
	return s, acc, found, err
}

// DeleteSession deletes the session stored under hash, if any.
func (db *DB) DeleteSession(hash []byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSessions).Delete(hash)
	})
}

// PurgeSessions deletes all sessions expired at now and returns their number.
func (db *DB) PurgeSessions(now time.Time) (int, error) {
	var expired [][]byte
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		err := b.ForEach(func(k, v []byte) error {
			var s model.Session
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			if !s.Expires.After(now) {
				expired = append(expired, bytes.Clone(k))
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, k := range expired {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
	return len(expired), err
}

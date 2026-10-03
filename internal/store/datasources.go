package store

import (
	"net/http"
	"slices"
	"strings"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// InUse details a data source that panels still use.
type InUse struct {
	Panels int      `json:"panels"`
	Sites  []string `json:"sites"`
}

// checkName fails if another data source has the name, ignoring case.
func checkName(tx *bolt.Tx, ds *model.DataSource) error {
	all, err := list[model.DataSource](tx, bucketSources, nil)
	if err != nil {
		return err
	}

	for _, other := range all {
		if other.ID != ds.ID && strings.EqualFold(other.Name, ds.Name) {
			return &apierr.Error{
				Status: http.StatusConflict,
				Code:   apierr.NameTaken,
				Fields: []apierr.Field{{
					Path: "name",
					Code: apierr.NameTaken,
				}},
			}
		}
	}
	return nil
}

// DataSources returns all data sources, ordered by ID.
func (db *DB) DataSources() ([]model.DataSource, error) {
	var all []model.DataSource
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		all, err = list[model.DataSource](tx, bucketSources, nil)
		return err
	})
	return all, err
}

// DataSource returns the data source with the ID.
func (db *DB) DataSource(id string) (*model.DataSource, error) {
	var ds model.DataSource
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return mustGet(tx, bucketSources, []byte(id), &ds)
	})
	return &ds, err
}

// CreateDataSource stores a new data source. Its ID is set by the caller,
// since sealed secrets are bound to it. Names are unique, ignoring case.
func (db *DB) CreateDataSource(ds *model.DataSource) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if err := checkName(tx, ds); err != nil {
			return err
		}

		ds.Revision = 1
		ds.CreatedAt = now()
		ds.UpdatedAt = ds.CreatedAt
		return put(tx, bucketSources, []byte(ds.ID), ds)
	})
}

// UpdateDataSource replaces a data source's name and configuration and
// increments its revision.
func (db *DB) UpdateDataSource(ds *model.DataSource) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		var old model.DataSource
		if err := mustGet(tx, bucketSources, []byte(ds.ID), &old); err != nil {
			return err
		}

		if err := checkName(tx, ds); err != nil {
			return err
		}

		ds.Type = old.Type
		ds.Revision = old.Revision + 1
		ds.CreatedAt = old.CreatedAt
		ds.UpdatedAt = now()
		return put(tx, bucketSources, []byte(ds.ID), ds)
	})
}

// DeleteDataSource deletes a data source. While panels use it, this is a
// conflict that details the number of panels and their sites.
func (db *DB) DeleteDataSource(id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		err := mustGet(tx, bucketSources, []byte(id), &model.DataSource{})
		if err != nil {
			return err
		}

		panels, err := list[model.Panel](tx, bucketPanels, nil)
		if err != nil {
			return err
		}

		var use InUse
		for _, p := range panels {
			if p.DataSource == id {
				use.Panels++
				if !slices.Contains(use.Sites, p.Site) {
					use.Sites = append(use.Sites, p.Site)
				}
			}
		}

		if use.Panels > 0 {
			return &apierr.Error{
				Status:  http.StatusConflict,
				Code:    apierr.DataSourceInUse,
				Details: use,
			}
		}
		return tx.Bucket(bucketSources).Delete([]byte(id))
	})
}

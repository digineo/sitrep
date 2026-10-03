package store

import (
	"cmp"
	"net/http"
	"slices"
	"uuid"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// panelKey is the key of a panel: below its site, so that a site's panels
// can be listed by prefix.
func panelKey(site, id string) []byte {
	return []byte(site + "/" + id)
}

func sortPanels(panels []model.Panel) {
	slices.SortStableFunc(panels, func(a, b model.Panel) int {
		return cmp.Or(cmp.Compare(a.Site, b.Site), cmp.Compare(a.Order, b.Order))
	})
}

// checkDataSource fails with a field error if the panel's data source does
// not exist.
func checkDataSource(tx *bolt.Tx, p *model.Panel) error {
	if tx.Bucket(bucketSources).Get([]byte(p.DataSource)) == nil {
		return apierr.Fields{{
			Path: "datasource",
			Code: apierr.NotFound,
		}}.Err()
	}
	return nil
}

// Panels returns the site's panels in display order.
func (db *DB) Panels(site string) ([]model.Panel, error) {
	var panels []model.Panel
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		err = mustGet(tx, bucketSites, []byte(site), &model.Site{})
		if err != nil {
			return err
		}
		panels, err = list[model.Panel](tx, bucketPanels, []byte(site+"/"))
		return err
	})

	sortPanels(panels)
	return panels, err
}

// Panel returns the site's panel with the ID.
func (db *DB) Panel(site, id string) (*model.Panel, error) {
	var p model.Panel
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return mustGet(tx, bucketPanels, panelKey(site, id), &p)
	})
	return &p, err
}

// CreatePanel stores a new panel after the site's other panels and sets its
// ID, order and revision.
func (db *DB) CreatePanel(p *model.Panel) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		err := mustGet(tx, bucketSites, []byte(p.Site), &model.Site{})
		if err != nil {
			return err
		}

		if err := checkDataSource(tx, p); err != nil {
			return err
		}

		panels, err := list[model.Panel](tx, bucketPanels, []byte(p.Site+"/"))
		if err != nil {
			return err
		}

		p.ID = uuid.NewV7().String()
		p.Order = 0
		for _, other := range panels {
			p.Order = max(p.Order, other.Order+1)
		}

		p.Revision = 1
		return put(tx, bucketPanels, panelKey(p.Site, p.ID), p)
	})
}

// UpdatePanel replaces a panel's editable fields and increments its
// revision.
func (db *DB) UpdatePanel(p *model.Panel) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		var old model.Panel
		err := mustGet(tx, bucketPanels, panelKey(p.Site, p.ID), &old)
		if err != nil {
			return err
		}

		if err := checkDataSource(tx, p); err != nil {
			return err
		}

		p.Order = old.Order
		p.Revision = old.Revision + 1
		return put(tx, bucketPanels, panelKey(p.Site, p.ID), p)
	})
}

// DeletePanel deletes the site's panel with the ID.
func (db *DB) DeletePanel(site, id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		err := mustGet(tx, bucketPanels, panelKey(site, id), &model.Panel{})
		if err != nil {
			return err
		}
		return tx.Bucket(bucketPanels).Delete(panelKey(site, id))
	})
}

// ReorderPanels puts the site's panels in the order of ids, which must list
// each of them once. The order is presentation only, so revisions stay.
func (db *DB) ReorderPanels(site string, ids []string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		err := mustGet(tx, bucketSites, []byte(site), &model.Site{})
		if err != nil {
			return err
		}

		panels, err := list[model.Panel](tx, bucketPanels, []byte(site+"/"))
		if err != nil {
			return err
		}

		if len(ids) != len(panels) {
			return apierr.New(http.StatusBadRequest, apierr.InvalidValue)
		}

		for _, p := range panels {
			p.Order = slices.Index(ids, p.ID)
			if p.Order < 0 {
				return apierr.New(http.StatusBadRequest, apierr.InvalidValue)
			}

			if err := put(tx, bucketPanels, panelKey(site, p.ID), p); err != nil {
				return err
			}
		}
		return nil
	})
}

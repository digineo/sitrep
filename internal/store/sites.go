package store

import (
	"bytes"
	"fmt"
	"net/http"
	"uuid"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// routeKey is the key of a route in the routes bucket. Path slugs,
// subdomain slugs and custom domains are separate namespaces.
func routeKey(r model.Route) []byte {
	if r.Mode == model.RouteCustom {
		return []byte(r.Mode + ":" + r.Domain)
	}
	return []byte(r.Mode + ":" + r.Slug)
}

// routeConflict reports a route that another site already uses, at the
// route's field.
func routeConflict(r model.Route) error {
	field := "route.slug"
	if r.Mode == model.RouteCustom {
		field = "route.domain"
	}
	return &apierr.Error{
		Status: http.StatusConflict,
		Code:   apierr.RouteConflict,
		Fields: []apierr.Field{{
			Path: field,
			Code: apierr.RouteConflict,
		}},
	}
}

// CreateSite stores a new site and sets its ID and timestamps. A route that
// another site already uses is a conflict.
func (db *DB) CreateSite(s *model.Site) error {
	return db.bolt.Update(func(tx *bolt.Tx) error { return createSite(tx, s) })
}

func createSite(tx *bolt.Tx, s *model.Site) error {
	routes := tx.Bucket(bucketRoutes)
	key := routeKey(s.Route)
	if routes.Get(key) != nil {
		return routeConflict(s.Route)
	}

	s.ID = uuid.NewV7().String()
	s.CreatedAt = now()
	s.UpdatedAt = s.CreatedAt
	if err := routes.Put(key, []byte(s.ID)); err != nil {
		return err
	}
	return put(tx, bucketSites, []byte(s.ID), s)
}

// Site returns the site with the ID.
func (db *DB) Site(id string) (*model.Site, error) {
	var site model.Site
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return mustGet(tx, bucketSites, []byte(id), &site)
	})
	return &site, err
}

// UpdateSite replaces a site's editable fields and sets its update time. A
// new route that another site already uses is a conflict.
func (db *DB) UpdateSite(s *model.Site) error {
	return db.bolt.Update(func(tx *bolt.Tx) error { return updateSite(tx, s) })
}

func updateSite(tx *bolt.Tx, s *model.Site) error {
	var old model.Site
	if err := mustGet(tx, bucketSites, []byte(s.ID), &old); err != nil {
		return err
	}

	oldKey, key := routeKey(old.Route), routeKey(s.Route)
	if string(oldKey) != string(key) {
		routes := tx.Bucket(bucketRoutes)
		if routes.Get(key) != nil {
			return routeConflict(s.Route)
		}

		if err := routes.Delete(oldKey); err != nil {
			return err
		}

		if err := routes.Put(key, []byte(s.ID)); err != nil {
			return err
		}
	}

	s.CreatedAt = old.CreatedAt
	s.UpdatedAt = now()
	return put(tx, bucketSites, []byte(s.ID), s)
}

// ImportSite stores an imported site with its panels, in display order,
// in one transaction. A site with an ID replaces that site's settings,
// except its availability, and all its panels; its incidents remain. A
// site without ID is created online. The panels get new IDs.
func (db *DB) ImportSite(s *model.Site, panels []model.Panel) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		if s.ID == "" {
			s.Availability = model.AvailabilityOnline
			if err := createSite(tx, s); err != nil {
				return err
			}
		} else {
			var old model.Site
			if err := mustGet(tx, bucketSites, []byte(s.ID), &old); err != nil {
				return err
			}

			s.Availability = old.Availability
			if err := updateSite(tx, s); err != nil {
				return err
			}

			err := deletePrefix(tx.Bucket(bucketPanels), []byte(s.ID+"/"))
			if err != nil {
				return err
			}
		}

		for i := range panels {
			p := &panels[i]
			if tx.Bucket(bucketSources).Get([]byte(p.DataSource)) == nil {
				return apierr.Fields{{
					Path: fmt.Sprintf("panels[%d].datasource", i),
					Code: apierr.NotFound,
				}}.Err()
			}

			p.ID = uuid.NewV7().String()
			p.Site = s.ID
			p.Order = i
			p.Revision = 1
			if err := put(tx, bucketPanels, panelKey(s.ID, p.ID), p); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteSite deletes a site, its panels and its incidents.
func (db *DB) DeleteSite(id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		var site model.Site
		if err := mustGet(tx, bucketSites, []byte(id), &site); err != nil {
			return err
		}

		for _, bucket := range [][]byte{bucketPanels, bucketIncidents} {
			if err := deletePrefix(tx.Bucket(bucket), []byte(id+"/")); err != nil {
				return err
			}
		}

		if err := tx.Bucket(bucketRoutes).Delete(routeKey(site.Route)); err != nil {
			return err
		}
		return tx.Bucket(bucketSites).Delete([]byte(id))
	})
}

// deletePrefix deletes the keys of b with prefix.
func deletePrefix(b *bolt.Bucket, prefix []byte) error {
	c := b.Cursor()
	for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Seek(prefix) {
		if err := b.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// SiteByRoute returns the site reachable by the route, or nil.
func (db *DB) SiteByRoute(r model.Route) (*model.Site, error) {
	var site *model.Site
	err := db.bolt.View(func(tx *bolt.Tx) error {
		id := tx.Bucket(bucketRoutes).Get(routeKey(r))
		if id == nil {
			return nil
		}

		site = &model.Site{}
		_, err := get(tx, bucketSites, id, site)
		return err
	})
	return site, err
}

// Sites returns all sites, ordered by ID.
func (db *DB) Sites() ([]model.Site, error) {
	var sites []model.Site
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		sites, err = list[model.Site](tx, bucketSites, nil)
		return err
	})
	return sites, err
}

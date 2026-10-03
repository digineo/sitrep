package store

import (
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
	return db.bolt.Update(func(tx *bolt.Tx) error {
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
	})
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
	return db.bolt.Update(func(tx *bolt.Tx) error {
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
	})
}

// DeleteSite deletes a site and its panels.
func (db *DB) DeleteSite(id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		var site model.Site
		if err := mustGet(tx, bucketSites, []byte(id), &site); err != nil {
			return err
		}

		panels, err := list[model.Panel](tx, bucketPanels, []byte(id+"/"))
		if err != nil {
			return err
		}

		for _, p := range panels {
			err := tx.Bucket(bucketPanels).Delete(panelKey(p.Site, p.ID))
			if err != nil {
				return err
			}
		}

		if err := tx.Bucket(bucketRoutes).Delete(routeKey(site.Route)); err != nil {
			return err
		}
		return tx.Bucket(bucketSites).Delete([]byte(id))
	})
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

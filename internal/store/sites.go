package store

import (
	"encoding/json"
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

// CreateSite stores a new site and sets its ID. A route that another site
// already uses is a conflict.
func (db *DB) CreateSite(s *model.Site) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		routes := tx.Bucket(bucketRoutes)
		key := routeKey(s.Route)
		if routes.Get(key) != nil {
			return apierr.New(http.StatusConflict, apierr.RouteConflict)
		}

		s.ID = uuid.NewV7().String()
		if err := routes.Put(key, []byte(s.ID)); err != nil {
			return err
		}
		return put(tx, bucketSites, []byte(s.ID), s)
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
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSites).ForEach(func(_, v []byte) error {
			var s model.Site
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			sites = append(sites, s)
			return nil
		})
	})
	return sites, err
}

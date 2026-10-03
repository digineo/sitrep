package store

import (
	"time"
	"uuid"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/model"
)

// incidentKey is the key of an incident: below its site, so that a site's
// incidents can be listed by prefix.
func incidentKey(site, id string) []byte {
	return []byte(site + "/" + id)
}

// CreateIncident stores a new incident of an existing site, with its
// updates sorted and checked, and sets its ID and timestamps.
func (db *DB) CreateIncident(inc *model.Incident) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		err := mustGet(tx, bucketSites, []byte(inc.Site), &model.Site{})
		if err != nil {
			return err
		}

		inc.Sort()
		if err := inc.Check(); err != nil {
			return err
		}

		inc.ID = uuid.NewV7().String()
		inc.CreatedAt = now()
		inc.UpdatedAt = inc.CreatedAt
		return put(tx, bucketIncidents, incidentKey(inc.Site, inc.ID), inc)
	})
}

// Incident returns the site's incident with the ID.
func (db *DB) Incident(site, id string) (*model.Incident, error) {
	var inc model.Incident
	err := db.bolt.View(func(tx *bolt.Tx) error {
		return mustGet(tx, bucketIncidents, incidentKey(site, id), &inc)
	})
	return &inc, err
}

// Incidents returns the incidents of an existing site, ordered by ID.
func (db *DB) Incidents(site string) ([]model.Incident, error) {
	var all []model.Incident
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		err = mustGet(tx, bucketSites, []byte(site), &model.Site{})
		if err != nil {
			return err
		}
		all, err = list[model.Incident](tx, bucketIncidents, []byte(site+"/"))
		return err
	})
	return all, err
}

// ChangeIncident applies change to the site's incident in one transaction,
// then sorts its updates, checks them and sets its update time. An incident
// left without updates is deleted, and nil is returned for it.
func (db *DB) ChangeIncident(
	site, id string,
	change func(*model.Incident) error,
) (*model.Incident, error) {
	var inc *model.Incident
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		inc = &model.Incident{}
		key := incidentKey(site, id)
		if err := mustGet(tx, bucketIncidents, key, inc); err != nil {
			return err
		}

		if err := change(inc); err != nil {
			return err
		}

		if len(inc.Updates) == 0 {
			inc = nil
			return tx.Bucket(bucketIncidents).Delete(key)
		}

		inc.Sort()
		if err := inc.Check(); err != nil {
			return err
		}

		inc.UpdatedAt = now()
		return put(tx, bucketIncidents, key, inc)
	})
	if err != nil {
		return nil, err
	}
	return inc, nil
}

// DeleteIncident deletes the site's incident with the ID.
func (db *DB) DeleteIncident(site, id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		key := incidentKey(site, id)
		if err := mustGet(tx, bucketIncidents, key, &model.Incident{}); err != nil {
			return err
		}
		return tx.Bucket(bucketIncidents).Delete(key)
	})
}

// PurgeIncidents deletes the incidents that their site's retention expires
// at now, and returns the IDs of the sites that lost incidents with the
// number of deleted incidents.
func (db *DB) PurgeIncidents(now time.Time) (map[string]int, error) {
	purged := map[string]int{}
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		sites, err := list[model.Site](tx, bucketSites, nil)
		if err != nil {
			return err
		}

		for _, site := range sites {
			if site.IncidentRetentionDays == 0 {
				continue
			}

			prefix := []byte(site.ID + "/")
			incidents, err := list[model.Incident](tx, bucketIncidents, prefix)
			if err != nil {
				return err
			}

			for _, inc := range incidents {
				if !inc.Expired(now, site.IncidentRetentionDays) {
					continue
				}

				key := incidentKey(site.ID, inc.ID)
				if err := tx.Bucket(bucketIncidents).Delete(key); err != nil {
					return err
				}

				purged[site.ID]++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return purged, nil
}

package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"uuid"

	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

var (
	errForbidden  = apierr.New(http.StatusForbidden, apierr.Forbidden)
	errOwnAccount = apierr.New(http.StatusConflict, apierr.OwnAccount)
)

// SignIn returns the account of someone the provider authenticated, with
// their display name, email (verified or empty) and time of sign-in
// updated. It reports whether the account is new, and the ID of the
// pending account with the email, if any: it is bound to the subject, or
// merged into the subject's account if there is one. A new account is the
// provider's first one, and then owner, or has no role. Accounts are few,
// so lookups scan them all.
func (db *DB) SignIn(
	provider, subject, name, email string,
) (acc model.Account, pending string, created bool, err error) {
	if subject == "" {
		// It would match every pending account.
		return acc, "", false, errors.New("sign-in without subject")
	}

	email = strings.ToLower(email)
	err = db.bolt.Update(func(tx *bolt.Tx) error {
		all, err := list[model.Account](tx, bucketAccounts, nil)
		if err != nil {
			return err
		}

		var bound, unbound *model.Account
		first := true
		for i := range all {
			a := &all[i]
			if a.Provider != provider {
				continue
			}

			first = false
			switch {
			case a.Subject == subject:
				bound = a
			case a.Subject == "" && email != "" && a.Email == email:
				unbound = a
			}
		}

		switch {
		case bound != nil && unbound != nil:
			pending = unbound.ID
			merge(bound, unbound)
			if err := tx.Bucket(bucketAccounts).Delete([]byte(unbound.ID)); err != nil {
				return err
			}
		case unbound != nil:
			pending = unbound.ID
			bound = unbound
			bound.Subject = subject
		case bound == nil:
			created = true
			bound = &model.Account{
				ID:        uuid.NewV7().String(),
				Provider:  provider,
				Subject:   subject,
				CreatedAt: now(),
			}
			if first {
				bound.Role = model.RoleOwner
			}
		}

		t := now()
		bound.DisplayName = name
		bound.Email = email
		bound.LastSignIn = &t
		acc = *bound
		return put(tx, bucketAccounts, []byte(acc.ID), acc)
	})
	return acc, pending, created, err
}

// merge adds the roles of from to a, keeping the higher role per scope.
func merge(a, from *model.Account) {
	higher := func(x, y model.Role) model.Role {
		if x.Includes(y) {
			return x
		}
		return y
	}

	a.Role = higher(a.Role, from.Role)
	for site, r := range from.Sites {
		if a.Sites == nil {
			a.Sites = map[string]model.Role{}
		}
		a.Sites[site] = higher(a.Sites[site], r)
	}
}

// Actor is the account that changes another one, and the role it needs
// for that: on Site, or anywhere without Site.
type Actor struct {
	ID   string
	Site string
	Role model.Role
}

// authorize fails unless the actor, as stored now, holds its role and is
// not the account a: nobody changes their own account. Checking within
// the change keeps concurrent ones from acting on outdated roles, e.g. two
// owners from demoting each other. A nil actor is the operator, who may
// change any account.
func (by *Actor) authorize(tx *bolt.Tx, a *model.Account) error {
	if by == nil {
		return nil
	}

	var acc model.Account
	found, err := get(tx, bucketAccounts, []byte(by.ID), &acc)
	switch {
	case err != nil:
		return err
	case !found || !acc.Can(by.Site, by.Role):
		return errForbidden
	case a.ID == by.ID:
		return errOwnAccount
	}
	return nil
}

// checkSites fails unless the sites the account has roles on exist.
func checkSites(tx *bolt.Tx, a *model.Account) error {
	for site := range a.Sites {
		if tx.Bucket(bucketSites).Get([]byte(site)) == nil {
			return errNotFound
		}
	}
	return nil
}

// save stores the account, or deletes it if it is pending without any
// role: it would only keep someone's email.
func save(tx *bolt.Tx, a model.Account) error {
	if a.Subject == "" && a.Role == model.RoleNone && len(a.Sites) == 0 {
		return tx.Bucket(bucketAccounts).Delete([]byte(a.ID))
	}
	return put(tx, bucketAccounts, []byte(a.ID), a)
}

// Grant applies change by the actor to the provider's account with the
// login, a subject or else an email, creating the account if there is
// none. A new account for an email is pending until its first sign-in.
// A pending account left without roles is deleted, or not created.
func (db *DB) Grant(
	by *Actor,
	provider, login string,
	byEmail bool,
	change func(*model.Account) error,
) (model.Account, error) {
	var acc model.Account
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		all, err := list[model.Account](tx, bucketAccounts, nil)
		if err != nil {
			return err
		}

		var matches []model.Account
		for _, a := range all {
			key := a.Subject
			if byEmail {
				key = a.Email
			}
			if a.Provider == provider && key == login {
				matches = append(matches, a)
			}
		}

		switch len(matches) {
		case 0:
			acc = model.Account{
				ID:          uuid.NewV7().String(),
				Provider:    provider,
				DisplayName: login,
				CreatedAt:   now(),
			}
			if byEmail {
				acc.Email = login
			} else {
				acc.Subject = login
			}
		case 1:
			acc = matches[0]
		default:
			return apierr.New(http.StatusConflict, apierr.AmbiguousEmail)
		}

		if err := by.authorize(tx, &acc); err != nil {
			return err
		}
		if err := change(&acc); err != nil {
			return err
		}
		if err := checkSites(tx, &acc); err != nil {
			return err
		}
		return save(tx, acc)
	})
	return acc, err
}

// Account returns the account with the ID, if there is one.
func (db *DB) Account(id string) (model.Account, bool, error) {
	var a model.Account
	var found bool
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		found, err = get(tx, bucketAccounts, []byte(id), &a)
		return err
	})
	return a, found, err
}

// Accounts returns all accounts, oldest first.
func (db *DB) Accounts() ([]model.Account, error) {
	var accounts []model.Account
	err := db.bolt.View(func(tx *bolt.Tx) (err error) {
		accounts, err = list[model.Account](tx, bucketAccounts, nil)
		return err
	})
	return accounts, err
}

// UpdateAccount applies change by the actor to the account with the ID. A
// pending account left without roles is deleted.
func (db *DB) UpdateAccount(
	by *Actor,
	id string,
	change func(*model.Account) error,
) (model.Account, error) {
	var acc model.Account
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		if err := mustGet(tx, bucketAccounts, []byte(id), &acc); err != nil {
			return err
		}
		if err := by.authorize(tx, &acc); err != nil {
			return err
		}
		if err := change(&acc); err != nil {
			return err
		}
		if err := checkSites(tx, &acc); err != nil {
			return err
		}
		return save(tx, acc)
	})
	return acc, err
}

// DeleteAccount deletes an account and its sessions, by the actor. It
// replaces the account's person in incidents by model.System.
func (db *DB) DeleteAccount(by *Actor, id string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		var acc model.Account
		if err := mustGet(tx, bucketAccounts, []byte(id), &acc); err != nil {
			return err
		}
		if err := by.authorize(tx, &acc); err != nil {
			return err
		}

		var keys [][]byte
		sessions := tx.Bucket(bucketSessions)
		err := sessions.ForEach(func(k, v []byte) error {
			var s model.Session
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			if s.Account == id {
				keys = append(keys, bytes.Clone(k))
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, k := range keys {
			if err := sessions.Delete(k); err != nil {
				return err
			}
		}

		if acc.Subject != "" {
			if err := replaceAuthor(tx, acc.Subject); err != nil {
				return err
			}
		}
		return tx.Bucket(bucketAccounts).Delete([]byte(id))
	})
}

// dropSite removes the roles on a site from all accounts, deleting pending
// accounts left without roles.
func dropSite(tx *bolt.Tx, site string) error {
	accounts, err := list[model.Account](tx, bucketAccounts, nil)
	if err != nil {
		return err
	}

	for _, a := range accounts {
		if _, ok := a.Sites[site]; !ok {
			continue
		}

		delete(a.Sites, site)
		if err := save(tx, a); err != nil {
			return err
		}
	}
	return nil
}

// accountsFromSessions migrates schema 1, where everyone who could sign in
// had every permission: each subject with an unexpired session becomes an
// owner, named after its newest session, and its sessions point to the
// account. Expired sessions are deleted.
func accountsFromSessions(tx *bolt.Tx) error {
	type legacy struct {
		Provider    string    `json:"provider"`
		Subject     string    `json:"subject"`
		DisplayName string    `json:"displayName"`
		Email       string    `json:"email"`
		Expires     time.Time `json:"expires"`
	}

	type entry struct {
		key     []byte
		session legacy
	}

	var entries []entry
	b := tx.Bucket(bucketSessions)
	err := b.ForEach(func(k, v []byte) error {
		var s legacy
		if err := json.Unmarshal(v, &s); err != nil {
			return err
		}
		entries = append(entries, entry{bytes.Clone(k), s})
		return nil
	})
	if err != nil {
		return err
	}

	accounts := map[[2]string]*model.Account{}
	newest := map[string]time.Time{}
	for _, e := range entries {
		s := e.session
		if !s.Expires.After(now()) {
			if err := b.Delete(e.key); err != nil {
				return err
			}
			continue
		}

		a := accounts[[2]string{s.Provider, s.Subject}]
		if a == nil {
			a = &model.Account{
				ID:        uuid.NewV7().String(),
				Provider:  s.Provider,
				Subject:   s.Subject,
				Role:      model.RoleOwner,
				CreatedAt: now(),
			}
			accounts[[2]string{s.Provider, s.Subject}] = a
		}

		if s.Expires.After(newest[a.ID]) {
			newest[a.ID] = s.Expires
			a.DisplayName = s.DisplayName
			a.Email = strings.ToLower(s.Email)
		}

		session := model.Session{
			Account: a.ID,
			Expires: s.Expires,
		}
		if err := put(tx, bucketSessions, e.key, session); err != nil {
			return err
		}
	}

	for _, a := range accounts {
		if err := put(tx, bucketAccounts, []byte(a.ID), a); err != nil {
			return err
		}
	}
	return nil
}

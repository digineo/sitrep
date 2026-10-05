package store

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/digineo/sitrep/internal/model"
)

func TestSignIn(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	ann, _, created, err := db.SignIn("oidc", "u-1", "Ann", "Ann@Example.com")
	require.NoError(err)
	assert.True(created)
	assert.Equal(model.RoleOwner, ann.Role, "the provider's first account")
	assert.Equal("ann@example.com", ann.Email)
	assert.NotNil(ann.LastSignIn)

	bob, _, created, err := db.SignIn("oidc", "u-2", "Bob", "")
	require.NoError(err)
	assert.True(created)
	assert.Equal(model.RoleNone, bob.Role)

	again, _, created, err := db.SignIn("oidc", "u-1", "Ann B.", "")
	require.NoError(err)
	assert.False(created)
	assert.Equal(ann.ID, again.ID)
	assert.Equal("Ann B.", again.DisplayName)
	assert.Empty(again.Email, "an unverified email is not kept")

	other, _, _, err := db.SignIn("basic", "u-1", "Ann", "")
	require.NoError(err)
	assert.NotEqual(ann.ID, other.ID, "accounts are per provider")
	assert.Equal(model.RoleOwner, other.Role, "a switched provider bootstraps again")

	got, found, err := db.Account(ann.ID)
	require.NoError(err)
	assert.True(found)
	assert.Equal(again, got)

	_, found, err = db.Account("nope")
	require.NoError(err)
	assert.False(found)

	_, _, _, err = db.SignIn("oidc", "", "Nobody", "")
	assert.Error(err, "an empty subject would match pending accounts")
}

func TestSignInBindsPendingAccounts(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	s1, s2 := newSite(t, db, "a"), newSite(t, db, "b")
	_, _, _, err := db.SignIn("oidc", "u-1", "Ann", "ann@example.com")
	require.NoError(err)

	grant := func(login string, change func(*model.Account)) model.Account {
		acc, err := db.Grant(nil, "oidc", login, true, func(a *model.Account) error {
			change(a)
			return nil
		})
		require.NoError(err)
		return acc
	}

	carl := grant("carl@example.com", func(a *model.Account) {
		a.Sites = map[string]model.Role{s1: model.RoleMaintainer}
	})
	assert.Empty(carl.Subject, "provisioned by email: pending")

	acc, pending, created, err := db.SignIn("oidc", "u-3", "Carl", "")
	require.NoError(err)
	assert.True(created, "never bound without a verified email")
	assert.Empty(pending)
	assert.Empty(acc.Sites)

	acc, pending, created, err = db.SignIn("oidc", "u-4", "Carl", "Carl@example.com")
	require.NoError(err)
	assert.False(created)
	assert.Equal(carl.ID, pending)
	assert.Equal(carl.ID, acc.ID, "bound by verified email")
	assert.Equal("u-4", acc.Subject)
	assert.Equal(carl.Sites, acc.Sites)

	// u-3 verifies the email it signed in without, which meanwhile was
	// provisioned again: the pending account merges into u-3's.
	dora := grant("dora@example.com", func(a *model.Account) {
		a.Sites = map[string]model.Role{
			s1: model.RoleResponder,
			s2: model.RoleMaintainer,
		}
	})
	_, err = db.Grant(nil, "oidc", "u-3", false, func(a *model.Account) error {
		a.Sites = map[string]model.Role{s1: model.RoleMaintainer}
		return nil
	})
	require.NoError(err)

	acc, pending, created, err = db.SignIn("oidc", "u-3", "Dora", "dora@example.com")
	require.NoError(err)
	assert.False(created)
	assert.Equal(dora.ID, pending)
	assert.NotEqual(dora.ID, acc.ID, "merged")
	assert.Equal(map[string]model.Role{
		s1: model.RoleMaintainer,
		s2: model.RoleMaintainer,
	}, acc.Sites, "the higher role per site wins")

	_, err = db.Grant(nil, "oidc", "dora@example.com", true, func(a *model.Account) error {
		assert.Equal(acc.ID, a.ID, "the pending account is gone")
		return nil
	})
	require.NoError(err)
}

func TestGrant(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	owner := func(a *model.Account) error {
		a.Role = model.RoleOwner
		return nil
	}

	ann, err := db.Grant(nil, "basic", "ann", false, owner)
	require.NoError(err)
	assert.Equal("ann", ann.Subject, "accounts named by subject are bound")
	assert.Equal(model.RoleOwner, ann.Role)

	acc, _, created, err := db.SignIn("basic", "ann", "Ann", "")
	require.NoError(err)
	assert.False(created)
	assert.Equal(ann.ID, acc.ID)

	_, _, _, err = db.SignIn("oidc", "u-1", "Ann", "ann@example.com")
	require.NoError(err)
	_, _, _, err = db.SignIn("oidc", "u-2", "Ann", "ann@example.com")
	require.NoError(err)
	_, err = db.Grant(nil, "oidc", "ann@example.com", true, owner)
	e := apiError(t, err)
	assert.Equal(http.StatusConflict, e.Status)
	assert.Equal("ambiguous_email", e.Code)

	failed := errors.New("failed")
	_, err = db.Grant(nil, "basic", "bob", false, func(*model.Account) error {
		return failed
	})
	assert.ErrorIs(err, failed)
	acc, _, created, err = db.SignIn("basic", "bob", "Bob", "")
	require.NoError(err)
	assert.True(created, "a failed change stores nothing")
	assert.Equal(model.RoleNone, acc.Role)
}

func TestMigrateSessions(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, path := openTemp(t)
	now := time.Now().UTC().Truncate(time.Second)
	legacy := map[string]map[string]any{
		"t-1": {"provider": "oidc", "subject": "u-1", "displayName": "Ann", "expires": now.Add(time.Minute)},
		"t-2": {"provider": "oidc", "subject": "u-1", "displayName": "Ann B.", "email": "Ann@example.com", "expires": now.Add(time.Hour)},
		"t-3": {"provider": "oidc", "subject": "u-2", "displayName": "Bob", "expires": now.Add(time.Minute)},
		"t-4": {"provider": "oidc", "subject": "u-9", "displayName": "Gone", "email": "gone@example.com", "expires": now.Add(-time.Minute)},
	}
	require.NoError(db.bolt.Update(func(tx *bolt.Tx) error {
		for token, s := range legacy {
			if err := put(tx, bucketSessions, []byte(token), s); err != nil {
				return err
			}
		}
		return put(tx, bucketMeta, keySchema, 1)
	}))
	require.NoError(db.Close())

	db, err := Open(path)
	require.NoError(err)
	defer func() { _ = db.Close() }()

	accounts := map[string]model.Account{}
	for _, token := range []string{"t-1", "t-2", "t-3"} {
		s, _, found, err := db.Session([]byte(token))
		require.NoError(err)
		require.True(found)
		assert.Equal(legacy[token]["expires"], s.Expires, token)

		acc, found, err := db.Account(s.Account)
		require.NoError(err)
		require.True(found, token)
		accounts[acc.Subject] = acc
	}

	_, _, found, err := db.Session([]byte("t-4"))
	require.NoError(err)
	assert.False(found, "expired sessions are deleted, and grant nothing")

	all, err := db.Accounts()
	require.NoError(err)
	require.Len(all, 2)
	require.Len(accounts, 2)
	ann, bob := accounts["u-1"], accounts["u-2"]
	assert.Equal(model.RoleOwner, ann.Role)
	assert.Equal(model.RoleOwner, bob.Role)
	assert.Equal("oidc", ann.Provider)
	assert.Equal("Ann B.", ann.DisplayName, "named after the newest session")
	assert.Equal("ann@example.com", ann.Email)

	carl, _, _, err := db.SignIn("oidc", "u-3", "Carl", "")
	require.NoError(err)
	assert.Equal(model.RoleNone, carl.Role, "migrated owners are the first")

	var version int
	require.NoError(db.bolt.View(func(tx *bolt.Tx) error {
		_, err := get(tx, bucketMeta, keySchema, &version)
		return err
	}))
	assert.Equal(schemaVersion, version)
}

func TestUpdateAndDeleteAccount(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	site := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	require.NoError(db.CreateSite(site))
	ann, _, _, err := db.SignIn("basic", "ann", "Ann", "")
	require.NoError(err)
	bob, _, _, err := db.SignIn("basic", "bob", "Bob", "")
	require.NoError(err)

	bob, err = db.UpdateAccount(nil, bob.ID, func(a *model.Account) error {
		a.Sites = map[string]model.Role{site.ID: model.RoleMaintainer}
		return nil
	})
	require.NoError(err)
	accounts, err := db.Accounts()
	require.NoError(err)
	assert.Equal([]model.Account{ann, bob}, accounts, "oldest first")

	failed := errors.New("failed")
	_, err = db.UpdateAccount(nil, bob.ID, func(a *model.Account) error {
		a.Role = model.RoleOwner
		return failed
	})
	assert.ErrorIs(err, failed)
	got, _, err := db.Account(bob.ID)
	require.NoError(err)
	assert.Equal(bob, got, "a failed change stores nothing")

	_, err = db.UpdateAccount(nil, "nope", func(*model.Account) error { return nil })
	assert.Equal(http.StatusNotFound, apiError(t, err).Status)

	require.NoError(db.DeleteSite(site.ID))
	got, _, err = db.Account(bob.ID)
	require.NoError(err)
	assert.Empty(got.Sites, "roles go with their site")

	expires := time.Now().Add(time.Hour)
	for token, account := range map[string]string{"b-1": bob.ID, "b-2": bob.ID, "a-1": ann.ID} {
		s := model.Session{Account: account, Expires: expires}
		require.NoError(db.CreateSession([]byte(token), s))
	}

	require.NoError(db.DeleteAccount(nil, bob.ID))
	_, found, err := db.Account(bob.ID)
	require.NoError(err)
	assert.False(found)
	for token, want := range map[string]bool{"b-1": false, "b-2": false, "a-1": true} {
		_, _, found, err := db.Session([]byte(token))
		require.NoError(err)
		assert.Equal(want, found, "sessions go with their account: %s", token)
	}

	assert.Equal(http.StatusNotFound, apiError(t, db.DeleteAccount(nil, bob.ID)).Status)
}

// newSite creates a path-mode site and returns its ID.
func newSite(t *testing.T, db *DB, slug string) string {
	t.Helper()
	site := &model.Site{Route: model.Route{Mode: model.RoutePath, Slug: slug}}
	require.NoError(t, db.CreateSite(site))
	return site.ID
}

func TestActor(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	ann, _, _, err := db.SignIn("basic", "ann", "Ann", "")
	require.NoError(err)
	bob, err := db.Grant(nil, "basic", "bob", false, func(a *model.Account) error {
		a.Role = model.RoleOwner
		return nil
	})
	require.NoError(err)

	demote := func(a *model.Account) error {
		a.Role = model.RoleNone
		return nil
	}
	byAnn := &Actor{ID: ann.ID, Role: model.RoleOwner}
	byBob := &Actor{ID: bob.ID, Role: model.RoleOwner}

	_, err = db.UpdateAccount(byAnn, ann.ID, demote)
	assert.Equal("own_account", apiError(t, err).Code)
	_, err = db.Grant(byAnn, "basic", "ann", false, demote)
	assert.Equal("own_account", apiError(t, err).Code)
	assert.Equal("own_account", apiError(t, db.DeleteAccount(byAnn, ann.ID)).Code)

	// The owners demote each other at once, both allowed by the roles
	// their requests started with: the second is no owner any more.
	_, err = db.UpdateAccount(byAnn, bob.ID, demote)
	require.NoError(err)
	_, err = db.UpdateAccount(byBob, ann.ID, demote)
	assert.Equal(http.StatusForbidden, apiError(t, err).Status)
	assert.Equal(http.StatusForbidden, apiError(t, db.DeleteAccount(byBob, ann.ID)).Status)
	got, _, err := db.Account(ann.ID)
	require.NoError(err)
	assert.Equal(model.RoleOwner, got.Role, "an owner remains")

	_, err = db.Grant(byAnn, "basic", "carl", false, func(a *model.Account) error {
		a.Sites = map[string]model.Role{"nope": model.RoleResponder}
		return nil
	})
	assert.Equal(http.StatusNotFound, apiError(t, err).Status, "roles only on existing sites")
	_, err = db.UpdateAccount(byAnn, bob.ID, func(a *model.Account) error {
		a.Sites = map[string]model.Role{"nope": model.RoleResponder}
		return nil
	})
	assert.Equal(http.StatusNotFound, apiError(t, err).Status, "roles only on existing sites")
}

func TestPendingAccountsWithoutRoles(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, _ := openTemp(t)
	s1, s2 := newSite(t, db, "a"), newSite(t, db, "b")
	setSites := func(sites map[string]model.Role) func(*model.Account) error {
		return func(a *model.Account) error {
			a.Sites = sites
			return nil
		}
	}
	exists := func(id string) bool {
		_, found, err := db.Account(id)
		require.NoError(err)
		return found
	}

	carl, err := db.Grant(nil, "oidc", "carl@example.com", true, setSites(nil))
	require.NoError(err)
	assert.False(exists(carl.ID), "not created without roles")

	responder := map[string]model.Role{s1: model.RoleResponder}
	carl, err = db.Grant(nil, "oidc", "carl@example.com", true, setSites(responder))
	require.NoError(err)
	require.True(exists(carl.ID))
	_, err = db.UpdateAccount(nil, carl.ID, setSites(nil))
	require.NoError(err)
	assert.False(exists(carl.ID), "deleted with its last role")

	both := map[string]model.Role{s1: model.RoleResponder, s2: model.RoleResponder}
	dora, err := db.Grant(nil, "oidc", "dora@example.com", true, setSites(both))
	require.NoError(err)
	require.NoError(db.DeleteSite(s1))
	assert.True(exists(dora.ID), "a role remains")
	require.NoError(db.DeleteSite(s2))
	assert.False(exists(dora.ID), "deleted with its last site")

	_, _, _, err = db.SignIn("oidc", "u-1", "Ann", "")
	require.NoError(err)
	bob, _, _, err := db.SignIn("oidc", "u-2", "Bob", "")
	require.NoError(err)
	require.Equal(model.RoleNone, bob.Role)
	_, err = db.UpdateAccount(nil, bob.ID, setSites(nil))
	require.NoError(err)
	assert.True(exists(bob.ID), "signed-in accounts stay")
}

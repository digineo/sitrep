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
	_, _, _, err := db.SignIn("oidc", "u-1", "Ann", "ann@example.com")
	require.NoError(err)

	grant := func(login string, change func(*model.Account)) model.Account {
		acc, err := db.Grant("oidc", login, true, func(a *model.Account) error {
			change(a)
			return nil
		})
		require.NoError(err)
		return acc
	}

	carl := grant("carl@example.com", func(a *model.Account) {
		a.Sites = map[string]model.Role{"s-1": model.RoleMaintainer}
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
			"s-1": model.RoleResponder,
			"s-2": model.RoleMaintainer,
		}
	})
	_, err = db.Grant("oidc", "u-3", false, func(a *model.Account) error {
		a.Sites = map[string]model.Role{"s-1": model.RoleMaintainer}
		return nil
	})
	require.NoError(err)

	acc, pending, created, err = db.SignIn("oidc", "u-3", "Dora", "dora@example.com")
	require.NoError(err)
	assert.False(created)
	assert.Equal(dora.ID, pending)
	assert.NotEqual(dora.ID, acc.ID, "merged")
	assert.Equal(map[string]model.Role{
		"s-1": model.RoleMaintainer,
		"s-2": model.RoleMaintainer,
	}, acc.Sites, "the higher role per site wins")

	_, err = db.Grant("oidc", "dora@example.com", true, func(a *model.Account) error {
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

	ann, err := db.Grant("basic", "ann", false, owner)
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
	_, err = db.Grant("oidc", "ann@example.com", true, owner)
	e := apiError(t, err)
	assert.Equal(http.StatusConflict, e.Status)
	assert.Equal("ambiguous_email", e.Code)

	failed := errors.New("failed")
	_, err = db.Grant("basic", "bob", false, func(*model.Account) error {
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

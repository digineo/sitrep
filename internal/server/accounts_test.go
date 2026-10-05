package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/model"
)

// grant changes the roles of the test provider's account with the subject.
func (f *fixture) grant(subject string, role model.Role, sites map[string]model.Role) {
	f.t.Helper()
	_, err := f.db.Grant(nil, "test", subject, false, func(a *model.Account) error {
		a.Role = role
		a.Sites = sites
		return nil
	})
	require.NoError(f.t, err)
}

// siteID returns the ID of the fixture's site with the route mode and slug.
func (f *fixture) siteID(mode, slug string) string {
	f.t.Helper()
	site, err := f.db.SiteByRoute(model.Route{Mode: mode, Slug: slug})
	require.NoError(f.t, err)
	require.NotNil(f.t, site)
	return site.ID
}

// TestAuthorization checks the role every admin route requires. Requests
// that pass go on to fail for other reasons, since their IDs and bodies
// are made up; deleting the site comes last.
func TestAuthorization(t *testing.T) {
	const (
		anyone     = model.RoleNone
		responder  = model.RoleResponder
		maintainer = model.RoleMaintainer
		admin      = model.RoleAdmin
		owner      = model.RoleOwner
	)
	routes := []struct {
		method, path string
		role         model.Role
	}{
		{"GET", "/api/admin/settings", maintainer},
		{"PUT", "/api/admin/settings", admin},
		{"GET", "/api/admin/version", anyone},
		{"GET", "/api/admin/directory", maintainer},
		{"GET", "/api/admin/accounts", owner},
		{"POST", "/api/admin/accounts", owner},
		{"PUT", "/api/admin/accounts/nope", owner},
		{"DELETE", "/api/admin/accounts/nope", owner},
		{"GET", "/api/admin/datasource-types", maintainer},
		{"GET", "/api/admin/datasources", maintainer},
		{"POST", "/api/admin/datasources", admin},
		{"POST", "/api/admin/datasources/test", admin},
		{"GET", "/api/admin/datasources/nope", maintainer},
		{"PUT", "/api/admin/datasources/nope", admin},
		{"DELETE", "/api/admin/datasources/nope", admin},
		{"POST", "/api/admin/datasources/nope/test", admin},
		{"GET", "/api/admin/datasources/nope/fake/x", maintainer},
		{"GET", "/api/admin/sites", anyone},
		{"POST", "/api/admin/sites", admin},
		{"POST", "/api/admin/sites/import", admin},
		{"GET", "/api/admin/sites/{site}", responder},
		{"PUT", "/api/admin/sites/{site}", maintainer},
		{"GET", "/api/admin/sites/{site}/export", maintainer},
		{"PUT", "/api/admin/sites/{site}/import", maintainer},
		{"GET", "/api/admin/sites/{site}/members", maintainer},
		{"POST", "/api/admin/sites/{site}/members", maintainer},
		{"PUT", "/api/admin/sites/{site}/members/nope", maintainer},
		{"DELETE", "/api/admin/sites/{site}/members/nope", maintainer},
		{"GET", "/api/admin/sites/{site}/preview", responder},
		{"GET", "/api/admin/sites/{site}/panels", responder},
		{"POST", "/api/admin/sites/{site}/panels", maintainer},
		{"PUT", "/api/admin/sites/{site}/panel-order", maintainer},
		{"POST", "/api/admin/sites/{site}/panel-preview", maintainer},
		{"GET", "/api/admin/sites/{site}/panels/nope", responder},
		{"PUT", "/api/admin/sites/{site}/panels/nope", maintainer},
		{"DELETE", "/api/admin/sites/{site}/panels/nope", maintainer},
		{"GET", "/api/admin/sites/{site}/incidents", responder},
		{"POST", "/api/admin/sites/{site}/incidents", responder},
		{"GET", "/api/admin/sites/{site}/incidents/nope", responder},
		{"PUT", "/api/admin/sites/{site}/incidents/nope", responder},
		{"DELETE", "/api/admin/sites/{site}/incidents/nope", responder},
		{"POST", "/api/admin/sites/{site}/incidents/nope/updates", responder},
		{"PUT", "/api/admin/sites/{site}/incidents/nope/updates/nope", responder},
		{"DELETE", "/api/admin/sites/{site}/incidents/nope/updates/nope", responder},
		{"POST", "/api/admin/markdown", responder},
		{"POST", "/api/admin/svg", maintainer},
		{"DELETE", "/api/admin/sites/{site}", admin},
	}

	cases := map[string]struct {
		role  model.Role
		sites func(acme, beta string) map[string]model.Role
	}{
		"no role":               {anyone, nil},
		"responder":             {anyone, func(acme, _ string) map[string]model.Role { return map[string]model.Role{acme: responder} }},
		"maintainer":            {anyone, func(acme, _ string) map[string]model.Role { return map[string]model.Role{acme: maintainer} }},
		"maintainer elsewhere":  {anyone, func(_, beta string) map[string]model.Role { return map[string]model.Role{beta: maintainer} }},
		"admin":                 {admin, nil},
		"owner":                 {owner, nil},
		"admin with site roles": {admin, func(acme, _ string) map[string]model.Role { return map[string]model.Role{acme: responder} }},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			acme := f.siteID(model.RoutePath, "acme")
			beta := f.siteID(model.RouteSubdomain, "beta")

			f.signIn("{}") // Ann, the owner
			f.signIn(`{"subject":"bob"}`)
			acc := model.Account{Role: tt.role}
			if tt.sites != nil {
				acc.Sites = tt.sites(acme, beta)
			}
			f.grant("bob", acc.Role, acc.Sites)

			for _, route := range routes {
				path := strings.ReplaceAll(route.path, "{site}", acme)
				site := ""
				if strings.Contains(route.path, "{site}") {
					site = acme
				}

				w := f.admin(route.method, path, nil)
				allowed := acc.Can(site, route.role)
				assert.Equal(
					t,
					allowed,
					w.Code != http.StatusForbidden,
					"%s %s: %d %s",
					route.method,
					route.path,
					w.Code,
					w.Body.String(),
				)
			}
		})
	}
}

func TestSiteListIsFiltered(t *testing.T) {
	f := newFixture(t)
	acme := f.siteID(model.RoutePath, "acme")
	assert.Len(t, decode[[]siteSummary](t, f.admin("GET", "/api/admin/sites", nil), 200), 3)

	f.signIn(`{"subject":"bob"}`)
	assert.Empty(t, decode[[]siteSummary](t, f.admin("GET", "/api/admin/sites", nil), 200))

	f.grant("bob", model.RoleNone, map[string]model.Role{acme: model.RoleResponder})
	sites := decode[[]siteSummary](t, f.admin("GET", "/api/admin/sites", nil), 200)
	require.Len(t, sites, 1)
	assert.Equal(t, acme, sites[0].ID)
}

func TestMembers(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	acme := f.siteID(model.RoutePath, "acme")
	path := "/api/admin/sites/" + acme + "/members"
	f.signIn(`{"subject":"ann","email":"ann@example.com"}`)
	assert.Empty(decode[[]member](t, f.admin("GET", path, nil), 200))

	w := f.admin("POST", path, obj{"login": " Bob@Example.com ", "role": "maintainer"})
	bob := decode[member](t, w, 200)
	assert.Equal("bob@example.com", bob.Login)
	assert.Equal(model.RoleMaintainer, bob.Role)
	assert.True(bob.Pending, "provisioned by email")

	for body, field := range map[string]obj{
		"login": {"login": "bob", "role": "maintainer"},
		"role":  {"login": "bob@example.com", "role": "admin"},
	} {
		w := f.admin("POST", path, field)
		assert.Equal(http.StatusBadRequest, w.Code, body)
		assert.Contains(w.Body.String(), `"path":"`+body+`"`)
	}

	w = f.admin("POST", path, obj{"login": "ann@example.com", "role": "responder"})
	assert.Equal(http.StatusConflict, w.Code)
	assert.Contains(w.Body.String(), "own_account")
	w = f.admin("POST", "/api/admin/sites/nope/members", obj{"login": "x@example.com", "role": "responder"})
	assert.Equal(http.StatusNotFound, w.Code)

	f.signIn(`{"subject":"u-bob","email":"bob@example.com"}`)
	w = f.admin("GET", path, nil)
	members := decode[[]member](t, w, 200)
	require.Len(members, 1)
	assert.Equal(bob.ID, members[0].ID)
	assert.False(members[0].Pending, "bound at sign-in")

	// Bob, a maintainer, adds Carl, changes his role and removes him.
	w = f.admin("POST", path, obj{"login": "carl@example.com", "role": "responder"})
	carl := decode[member](t, w, 200)
	w = f.admin("PUT", path+"/"+carl.ID, obj{"role": "maintainer"})
	assert.Equal(model.RoleMaintainer, decode[member](t, w, 200).Role)
	w = f.admin("PUT", path+"/"+carl.ID, obj{"role": "owner"})
	assert.Equal(http.StatusBadRequest, w.Code)
	w = f.admin("PUT", path+"/"+bob.ID, obj{"role": "responder"})
	assert.Equal(http.StatusConflict, w.Code, "not oneself")
	assert.Equal(http.StatusNoContent, f.admin("DELETE", path+"/"+carl.ID, nil).Code)
	w = f.admin("PUT", path+"/"+carl.ID, obj{"role": "maintainer"})
	assert.Equal(http.StatusNotFound, w.Code, "only members")
	assert.Equal(http.StatusNotFound, f.admin("DELETE", path+"/"+carl.ID, nil).Code)
	w = f.admin("DELETE", path+"/"+bob.ID, nil)
	assert.Equal(http.StatusConflict, w.Code, "not oneself")
}

func TestAccounts(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	acme := f.siteID(model.RoutePath, "acme")
	f.signIn(`{"subject":"ann","email":"ann@example.com"}`)
	_, _, _, err := f.db.SignIn("basic", "old", "Old", "")
	require.NoError(err)

	w := f.admin("POST", "/api/admin/accounts", obj{"login": "carl@example.com", "role": "admin"})
	carl := decode[accountView](t, w, 200)
	assert.Equal(model.RoleAdmin, carl.Role)
	assert.True(carl.Pending)

	w = f.admin("POST", "/api/admin/accounts", obj{"login": "ann@example.com", "role": "admin"})
	assert.Equal(http.StatusConflict, w.Code, "owners do not demote themselves")
	for _, role := range []string{"maintainer", ""} {
		w = f.admin("POST", "/api/admin/accounts", obj{"login": "dora@example.com", "role": role})
		assert.Equal(http.StatusBadRequest, w.Code, "role %q", role)
	}

	accounts := decode[[]accountView](t, f.admin("GET", "/api/admin/accounts", nil), 200)
	require.Len(accounts, 3)
	stale := map[string]bool{}
	for _, a := range accounts {
		stale[a.DisplayName] = a.Stale
	}
	assert.Equal(map[string]bool{
		"Ann":              false,
		"Old":              true,
		"carl@example.com": false,
	}, stale, "accounts of another provider are stale")

	path := "/api/admin/accounts/" + carl.ID
	w = f.admin("PUT", path, obj{"role": "", "sites": obj{acme: "responder"}})
	carl = decode[accountView](t, w, 200)
	assert.Equal(model.RoleNone, carl.Role)
	assert.Equal(map[string]model.Role{acme: model.RoleResponder}, carl.Sites)

	w = f.admin("PUT", path, obj{"role": "owner", "sites": obj{acme: "admin"}})
	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Contains(w.Body.String(), `{"path":"sites.`+acme+`","code":"invalid_value"}`)
	w = f.admin("PUT", path, obj{"role": "owner", "sites": obj{"nope": "responder"}})
	assert.Equal(http.StatusNotFound, w.Code)

	ann := accounts[0]
	w = f.admin("PUT", "/api/admin/accounts/"+ann.ID, obj{"role": "owner"})
	assert.Equal(http.StatusConflict, w.Code)
	w = f.admin("DELETE", "/api/admin/accounts/"+ann.ID, nil)
	assert.Equal(http.StatusConflict, w.Code)

	assert.Equal(http.StatusNoContent, f.admin("DELETE", path, nil).Code)
	assert.Len(decode[[]accountView](t, f.admin("GET", "/api/admin/accounts", nil), 200), 2)
}

// directoryProvider is a testProvider whose users are listed.
type directoryProvider struct {
	testProvider
	users []string
}

func (d directoryProvider) Users() ([]string, error) {
	return d.users, nil
}

func TestDirectory(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	w := f.admin("GET", "/api/admin/directory", nil)
	assert.Equal(http.StatusNotFound, w.Code, "the test provider has no directory")

	useDirectory := func(users ...string) {
		p := directoryProvider{users: users}
		core := auth.NewCore(xlog.NewDiscard(), f.db, "test", p, time.Hour, false)
		f.srv = newServer(xlog.NewDiscard(), f.cfg, f.db, core, f.poller, testAssets())
	}

	// The signed-in account, Ann, is not listed.
	useDirectory()
	w = f.admin("GET", "/api/admin/directory", nil)
	assert.JSONEq(`[]`, w.Body.String(), "an empty directory, unlike none")
	accounts := decode[[]accountView](t, f.admin("GET", "/api/admin/accounts", nil), 200)
	assert.True(accounts[0].Stale, "not in the empty directory")

	useDirectory("ann", "bob")
	assert.Equal([]string{"ann", "bob"}, decode[[]string](t, f.admin("GET", "/api/admin/directory", nil), 200))

	path := "/api/admin/sites/" + f.siteID(model.RoutePath, "acme") + "/members"
	w = f.admin("POST", path, obj{"login": "carl", "role": "responder"})
	assert.Equal(http.StatusBadRequest, w.Code)
	assert.Contains(w.Body.String(), "unknown_user")

	w = f.admin("POST", path, obj{"login": "bob", "role": "responder"})
	bob := decode[member](t, w, 200)
	assert.Equal("bob", bob.Login)
	assert.False(bob.Pending, "named by subject: bound")
}

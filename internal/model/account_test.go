package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountCan(t *testing.T) {
	maintainer := Account{Sites: map[string]Role{
		"s-1": RoleMaintainer,
		"s-2": RoleResponder,
	}}
	responder := Account{Sites: map[string]Role{"s-2": RoleResponder}}
	tests := []struct {
		name    string
		account Account
		site    string
		role    Role
		want    bool
	}{
		{"nobody signed in", Account{}, "", RoleNone, true},
		{"nobody on a site", Account{}, "s-1", RoleResponder, false},
		{"nobody anywhere", Account{}, "", RoleResponder, false},
		{"maintainer on own site", maintainer, "s-1", RoleMaintainer, true},
		{"maintainer responds on own site", maintainer, "s-1", RoleResponder, true},
		{"responder role on other site", maintainer, "s-2", RoleMaintainer, false},
		{"maintainer on unknown site", maintainer, "s-3", RoleResponder, false},
		{"maintainer anywhere", maintainer, "", RoleMaintainer, true},
		{"maintainer is no admin", maintainer, "", RoleAdmin, false},
		{"responder anywhere", responder, "", RoleResponder, true},
		{"responder maintains nothing", responder, "", RoleMaintainer, false},
		{"admin on every site", Account{Role: RoleAdmin}, "s-3", RoleMaintainer, true},
		{"admin is no owner", Account{Role: RoleAdmin}, "", RoleOwner, false},
		{"owner", Account{Role: RoleOwner}, "", RoleOwner, true},
		{"unknown role", Account{Role: "root"}, "", RoleResponder, false},
		{"nobody holds an unknown role", Account{}, "", "Maintainer", false},
		{"owner holds no unknown role", Account{Role: RoleOwner}, "s-1", "Maintainer", false},
	}
	for _, tt := range tests {
		got := tt.account.Can(tt.site, tt.role)
		assert.Equal(t, tt.want, got, tt.name)
	}
}

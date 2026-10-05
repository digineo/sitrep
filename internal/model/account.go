package model

import (
	"slices"
	"time"
)

// Role is what an account may do, on the instance or on one site.
type Role string

// Roles, from least to most. Each role includes the ones before it.
const (
	RoleNone       Role = ""
	RoleResponder  Role = "responder"
	RoleMaintainer Role = "maintainer"
	RoleAdmin      Role = "admin"
	RoleOwner      Role = "owner"
)

var roles = []Role{RoleNone, RoleResponder, RoleMaintainer, RoleAdmin, RoleOwner}

// The roles an account can hold on the instance and on a site.
var (
	InstanceRoles = []Role{RoleNone, RoleAdmin, RoleOwner}
	SiteRoles     = []Role{RoleResponder, RoleMaintainer}
)

// Includes reports whether r grants everything other grants. Nothing
// includes an unknown role.
func (r Role) Includes(other Role) bool {
	i := slices.Index(roles, other)
	return i >= 0 && slices.Index(roles, r) >= i
}

// Account is someone who may sign in to the admin console. An account
// provisioned by email is pending, without subject, until its first
// sign-in.
type Account struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Subject  string `json:"subject,omitempty"`
	// Email is verified by the identity provider, and lowercase.
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName"`
	// Role is the role on the instance: none, admin or owner.
	Role Role `json:"role,omitempty"`
	// Sites maps site IDs to the roles on them: responder or maintainer.
	Sites      map[string]Role `json:"sites,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	LastSignIn *time.Time      `json:"lastSignIn,omitempty"`
}

// Can reports whether the account holds at least role on the site, or,
// without site, anywhere.
func (a *Account) Can(site string, role Role) bool {
	if a.Role.Includes(role) {
		return true
	}
	if site != "" {
		return a.Sites[site].Includes(role)
	}
	for _, r := range a.Sites {
		if r.Includes(role) {
			return true
		}
	}
	return false
}

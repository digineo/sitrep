/** Role is what an account may do, on the instance or on one site. */
export type Role = "" | "responder" | "maintainer" | "admin" | "owner"

/** The roles, from least to most; each one includes the ones before. */
const ranks: Role[] = ["", "responder", "maintainer", "admin", "owner"]

/** instanceRoles and siteRoles are the roles accounts can hold. */
export const instanceRoles: Role[] = ["", "admin", "owner"]
export const siteRoles: Role[] = ["responder", "maintainer"]

export interface Roles {
  role:  Role
  sites: Record<string, Role>
}

/** includes reports whether role grants other; nothing includes an unknown role. */
function includes(role: Role, other: Role): boolean {
  const i = ranks.indexOf(other)
  return i >= 0 && ranks.indexOf(role) >= i
}

/**
 * can reports whether the roles include role on the site, or, without
 * site, anywhere. It mirrors the server's check.
 */
export function can(roles: Roles | null, site: string, role: Role): boolean {
  if (!roles) {
    return false
  }
  if (includes(roles.role, role)) {
    return true
  }
  if (site) {
    return includes(roles.sites[site] ?? "", role)
  }
  return Object.values(roles.sites).some(r => includes(r, role))
}

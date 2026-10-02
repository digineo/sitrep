package model

import (
	"net/url"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether s is a lowercase URL slug of 1 to 63 characters.
func ValidSlug(s string) bool {
	return len(s) <= 63 && slugPattern.MatchString(s)
}

// ValidDomain reports whether s is a lowercase ASCII hostname with at least
// two labels, without port, trailing dot or wildcard.
func ValidDomain(s string) bool {
	if len(s) > 253 {
		return false
	}

	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}

	for _, l := range labels {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}

		for _, c := range l {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

// NormalizeOrigin returns the canonical form of an origin
// "scheme://host[:port]": lowercase, without trailing slash and default
// port. It rejects other schemes than http and https, paths, userinfo,
// queries, fragments and wildcards.
func NormalizeOrigin(s string) (string, bool) {
	u, err := url.Parse(strings.TrimSuffix(strings.ToLower(s), "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.Path != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		strings.Contains(u.Host, "*") || strings.HasSuffix(u.Host, ":") {
		return "", false
	}

	host := u.Host
	p := u.Port()
	if (u.Scheme == "http" && p == "80") || (u.Scheme == "https" && p == "443") {
		host = strings.TrimSuffix(host, ":"+p)
	}
	return u.Scheme + "://" + host, true
}

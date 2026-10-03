package model

import (
	"slices"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/i18n"
)

// Length limits of texts, in characters per language.
const (
	maxName        = 200
	maxDescription = 2000
	maxQuery       = 10000
	maxUnit        = 32
	maxLegend      = 200
)

// Route modes of a site.
const (
	RoutePath      = "path"
	RouteSubdomain = "subdomain"
	RouteCustom    = "custom"
)

// SiteThemes are the color schemes a site can choose; "inherit" uses the
// instance default.
var SiteThemes = []string{"inherit", "light", "dark", "system"}

// reservedSlugs are path segments the apex serves itself.
var reservedSlugs = []string{"admin", "api", "auth", "assets", "healthz", "tls"}

// Site is a public status page.
type Site struct {
	ID        string    `json:"id"`
	Name      Text      `json:"name"`
	Languages Languages `json:"languages"`
	Timezone  string    `json:"timezone"`
	Route     Route     `json:"route"`
	Theme     string    `json:"theme"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Route says how a site is reached: below a base domain path, as subdomain
// of every base domain, or on its own domain.
type Route struct {
	Mode   string `json:"mode"`
	Slug   string `json:"slug,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// Normalize trims the texts, fills defaults and drops the route field the
// mode does not use.
func (s *Site) Normalize() {
	s.Name = s.Name.Normalize()

	if s.Timezone == "" {
		s.Timezone = "UTC"
	}

	if s.Theme == "" {
		s.Theme = "inherit"
	}

	if s.Route.Mode == RouteCustom {
		s.Route.Slug = ""
	} else {
		s.Route.Domain = ""
	}
}

// Validate checks a normalized site. Custom domains must not be served as
// base domain or subdomain site instead.
func (s *Site) Validate(baseDomains []string) error {
	var f apierr.Fields
	s.Languages.validate(&f, "languages")
	validateText(&f, "name", s.Name, s.Languages, true, maxName)

	if !ValidTimezone(s.Timezone) {
		f.Add("timezone", apierr.InvalidTimezone)
	}

	switch s.Route.Mode {
	case RoutePath, RouteSubdomain:
		switch {
		case !ValidSlug(s.Route.Slug):
			f.Add("route.slug", apierr.InvalidSlug)
		case s.Route.Mode == RoutePath && reservedSlug(s.Route.Slug):
			f.Add("route.slug", apierr.SlugReserved)
		}
	case RouteCustom:
		switch {
		case !ValidDomain(s.Route.Domain):
			f.Add("route.domain", apierr.InvalidDomain)
		case shadowed(s.Route.Domain, baseDomains):
			f.Add("route.domain", apierr.DomainReserved)
		}
	default:
		f.Add("route.mode", apierr.InvalidValue)
	}

	if !slices.Contains(SiteThemes, s.Theme) {
		f.Add("theme", apierr.InvalidValue)
	}
	return f.Err()
}

// ValidTimezone reports whether tz names a zone of the tz database.
func ValidTimezone(tz string) bool {
	if tz == "" || tz == "Local" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// reservedSlug reports whether a path-mode site cannot use slug: the apex
// serves the path itself, or it looks like a language or legal page.
func reservedSlug(slug string) bool {
	if slices.Contains(reservedSlugs, slug) || i18n.ValidCode(slug) {
		return true
	}
	for _, lang := range i18n.Supported() {
		if c := i18n.Get(lang); slug == c.Imprint || slug == c.Privacy {
			return true
		}
	}
	return false
}

// shadowed reports whether domain is a base domain or a single-label
// subdomain of one, which routing resolves before custom domains.
func shadowed(domain string, baseDomains []string) bool {
	for _, base := range baseDomains {
		label, ok := strings.CutSuffix(domain, "."+base)
		if domain == base || ok && !strings.Contains(label, ".") {
			return true
		}
	}
	return false
}

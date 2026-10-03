package model

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/svg"
)

// Length limits of texts, in characters per language.
const (
	maxName        = 200
	maxDescription = 2000
	maxQuery       = 10000
	maxUnit        = 32
	maxLegend      = 200
)

const maxRetentionDays = 36500

// Route modes of a site.
const (
	RoutePath      = "path"
	RouteSubdomain = "subdomain"
	RouteCustom    = "custom"
)

// Availabilities of a site. Visitors see neither offline nor paused sites,
// and paused sites are not polled either.
const (
	AvailabilityOnline  = "online"
	AvailabilityOffline = "offline"
	AvailabilityPaused  = "paused"
)

var (
	availabilities = []string{
		AvailabilityOnline,
		AvailabilityOffline,
		AvailabilityPaused,
	}
	colorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)
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
	// BrandColor colors the header and footer of the site, as "#rrggbb".
	BrandColor string `json:"brandColor,omitempty"`
	// Logo is the sanitized SVG source of the site's logo.
	Logo  string `json:"logo,omitempty"`
	Legal Legal  `json:"legal"`
	// AllowedOrigins may read incidents.json from other sites (CORS).
	AllowedOrigins []string `json:"allowedOrigins"`
	// IncidentRetentionDays deletes finished incidents that long after
	// their last activity; 0 keeps them forever.
	IncidentRetentionDays int       `json:"incidentRetentionDays"`
	Availability          string    `json:"availability"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// Route says how a site is reached: below a base domain path, as subdomain
// of every base domain, or on its own domain.
type Route struct {
	Mode   string `json:"mode"`
	Slug   string `json:"slug,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// Online reports whether visitors can see the site.
func (s *Site) Online() bool {
	return s.Availability != AvailabilityOffline &&
		s.Availability != AvailabilityPaused
}

// Normalize trims the texts, fills defaults, drops the fields the route
// and legal page modes do not use, sanitizes the logo and writes allowed
// origins in canonical form, without blank entries and duplicates. A logo
// that cannot be sanitized stays as it is, for Validate to report.
func (s *Site) Normalize() {
	s.Name = s.Name.Normalize()
	s.BrandColor = strings.ToLower(strings.TrimSpace(s.BrandColor))
	s.Logo = strings.TrimSpace(s.Logo)
	if logo, code := SanitizeLogo(s.Logo); code == "" {
		s.Logo = logo
	}

	s.Legal = s.Legal.normalize(LegalInherit)

	origins := []string{}
	for _, o := range s.AllowedOrigins {
		if o = strings.TrimSpace(o); o == "" {
			continue
		}

		if n, ok := NormalizeOrigin(o); ok {
			o = n
		}
		if !slices.Contains(origins, o) {
			origins = append(origins, o)
		}
	}

	s.AllowedOrigins = origins

	if s.Timezone == "" {
		s.Timezone = "UTC"
	}

	if s.Theme == "" {
		s.Theme = "inherit"
	}

	if s.Availability == "" {
		s.Availability = AvailabilityOnline
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

	if s.BrandColor != "" && !colorPattern.MatchString(s.BrandColor) {
		f.Add("brandColor", apierr.InvalidValue)
	}

	if _, code := SanitizeLogo(s.Logo); s.Logo != "" && code != "" {
		f.Add("logo", code)
	}

	s.Legal.validate(&f, "legal", s.Languages, siteLegalModes)

	for i, o := range s.AllowedOrigins {
		if _, ok := NormalizeOrigin(o); !ok {
			f.Add(fmt.Sprintf("allowedOrigins[%d]", i), apierr.InvalidOrigin)
		}
	}

	if s.IncidentRetentionDays < 0 || s.IncidentRetentionDays > maxRetentionDays {
		f.Add("incidentRetentionDays", apierr.OutOfRange)
	}

	if !slices.Contains(availabilities, s.Availability) {
		f.Add("availability", apierr.InvalidValue)
	}
	return f.Err()
}

// SanitizeLogo returns the sanitized SVG source of a logo, or the error
// code if it cannot be used.
func SanitizeLogo(src string) (string, string) {
	logo, err := svg.Sanitize(src)
	switch {
	case errors.Is(err, svg.ErrTooLarge):
		return "", apierr.SVGTooLarge
	case err != nil:
		return "", apierr.InvalidSVG
	}
	return logo, ""
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

package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

func fieldCodes(t *testing.T, err error) map[string]string {
	t.Helper()
	if err == nil {
		return nil
	}

	e, ok := errors.AsType[*apierr.Error](err)
	require.True(t, ok, err)
	out := map[string]string{}
	for _, f := range e.Fields {
		out[f.Path] = f.Code
	}
	return out
}

var bases = []string{"status.example.com", "sitrep.localhost"}

func validSite() Site {
	return Site{
		Name: Text{"en": " Acme "},
		Languages: Languages{
			Enabled: []string{"en"},
			Primary: "en",
		},
		Route: Route{
			Mode:   RoutePath,
			Slug:   "acme",
			Domain: "dropped.example.com",
		},
	}
}

func TestSiteNormalize(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	s := validSite()
	s.Normalize()
	require.NoError(s.Validate(bases))
	assert.Equal(Text{"en": "Acme"}, s.Name)
	assert.Equal("UTC", s.Timezone)
	assert.Equal("inherit", s.Theme)
	route := Route{
		Mode: RoutePath,
		Slug: "acme",
	}
	assert.Equal(route, s.Route)
	assert.Equal(AvailabilityOnline, s.Availability)
	legal := Legal{
		Imprint: LegalPage{Mode: LegalInherit},
		Privacy: LegalPage{Mode: LegalInherit},
	}
	assert.Equal(legal, s.Legal)
	assert.True(s.Online())

	s.Route = Route{
		Mode:   RouteCustom,
		Slug:   "dropped",
		Domain: "status.acme.com",
	}
	s.Normalize()
	route = Route{
		Mode:   RouteCustom,
		Domain: "status.acme.com",
	}
	assert.Equal(route, s.Route)
}

func TestSiteValidate(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Site)
		want   map[string]string
	}{
		{"name required in primary", func(s *Site) { s.Name = Text{"de": "Acme"} }, map[string]string{"name.en": "required"}},
		{"name too long", func(s *Site) { s.Name["de"] = strings.Repeat("ä", 201) }, map[string]string{"name.de": "too_long"}},
		{"name at the limit", func(s *Site) { s.Name["de"] = strings.Repeat("ä", 200) }, nil},
		{"name in a malformed language", func(s *Site) { s.Name["EN"] = "x" }, map[string]string{"name.EN": "unsupported_language"}},
		{"name in a disabled language", func(s *Site) { s.Name["fr"] = "x" }, nil},
		{"time zone", func(s *Site) { s.Timezone = "Europe/Berlin" }, nil},
		{"unknown time zone", func(s *Site) { s.Timezone = "Mars/Olympus" }, map[string]string{"timezone": "invalid_timezone"}},
		{"local time zone", func(s *Site) { s.Timezone = "Local" }, map[string]string{"timezone": "invalid_timezone"}},
		{"theme", func(s *Site) { s.Theme = "dark" }, nil},
		{"unknown theme", func(s *Site) { s.Theme = "blue" }, map[string]string{"theme": "invalid_value"}},
		{"mode", func(s *Site) { s.Route.Mode = "port" }, map[string]string{"route.mode": "invalid_value"}},
		{"slug", func(s *Site) { s.Route.Slug = "Acme" }, map[string]string{"route.slug": "invalid_slug"}},
		{"reserved slug", func(s *Site) { s.Route.Slug = "admin" }, map[string]string{"route.slug": "slug_reserved"}},
		{"language slug", func(s *Site) { s.Route.Slug = "fr" }, map[string]string{"route.slug": "slug_reserved"}},
		{"region slug", func(s *Site) { s.Route.Slug = "pt-br" }, map[string]string{"route.slug": "slug_reserved"}},
		{"legal slug", func(s *Site) { s.Route.Slug = "impressum" }, map[string]string{"route.slug": "slug_reserved"}},
		{"reserved slug as subdomain", func(s *Site) { s.Route = Route{Mode: RouteSubdomain, Slug: "admin"} }, nil},
		{"domain", func(s *Site) { s.Route = Route{Mode: RouteCustom, Domain: "Status.acme.com"} }, map[string]string{"route.domain": "invalid_domain"}},
		{"base domain", func(s *Site) { s.Route = Route{Mode: RouteCustom, Domain: "status.example.com"} },
			map[string]string{"route.domain": "domain_reserved"}},
		{"subdomain of a base domain", func(s *Site) { s.Route = Route{Mode: RouteCustom, Domain: "acme.sitrep.localhost"} },
			map[string]string{"route.domain": "domain_reserved"}},
		{"deeper below a base domain", func(s *Site) { s.Route = Route{Mode: RouteCustom, Domain: "a.acme.sitrep.localhost"} }, nil},
		{"languages", func(s *Site) { s.Languages.Enabled = []string{"de", "en"} }, nil},
		{"primary not enabled", func(s *Site) { s.Languages.Primary = "de" },
			map[string]string{"languages.primary": "primary_not_enabled", "name.de": "required"}},
		{"brand color", func(s *Site) { s.BrandColor = "#0a1b2c" }, nil},
		{"short brand color", func(s *Site) { s.BrandColor = "#abc" }, map[string]string{"brandColor": "invalid_value"}},
		{"named brand color", func(s *Site) { s.BrandColor = "red" }, map[string]string{"brandColor": "invalid_value"}},
		{"logo", func(s *Site) { s.Logo = `<svg xmlns="http://www.w3.org/2000/svg"></svg>` }, nil},
		{"logo that is no SVG", func(s *Site) { s.Logo = "<html></html>" }, map[string]string{"logo": "invalid_svg"}},
		{"availability", func(s *Site) { s.Availability = AvailabilityPaused }, nil},
		{"unknown availability", func(s *Site) { s.Availability = "hidden" }, map[string]string{"availability": "invalid_value"}},
		{"unknown legal mode", func(s *Site) { s.Legal.Privacy.Mode = "pdf" }, map[string]string{"legal.privacy.mode": "invalid_value"}},
		{"legal text", func(s *Site) { s.Legal.Imprint = LegalPage{Mode: LegalText, Text: Text{"en": "Acme Inc."}} }, nil},
	}
	for _, tt := range tests {
		s := validSite()
		s.Normalize()
		tt.modify(&s)
		assert.Equal(t, tt.want, fieldCodes(t, s.Validate(bases)), tt.name)
	}
}

func TestTextMissing(t *testing.T) {
	assert := assert.New(t)

	langs := Languages{
		Enabled: []string{"en", "de"},
		Primary: "en",
	}
	text := Text{
		"en": "a",
		"de": "b",
	}
	assert.False(text.Missing(langs))
	assert.True(Text{"en": "a"}.Missing(langs))
	assert.True(Text{"de": "b"}.Missing(langs))
	assert.False(Text(nil).Missing(langs), "an empty optional text misses nothing")

	text = Text{
		"en": "a",
		"fr": "c",
	}
	enOnly := Languages{
		Enabled: []string{"en"},
		Primary: "en",
	}
	assert.False(text.Missing(enOnly))

	text = Text{
		"en": " a ",
		"de": "  ",
	}
	assert.Equal(Text{"en": "a"}, text.Normalize())
	assert.Nil(Text{"en": " "}.Normalize())
}

func TestSiteOrigins(t *testing.T) {
	s := validSite()
	s.AllowedOrigins = []string{
		" HTTPS://Shop.Example.com:443/ ",
		"",
		"https://shop.example.com",
		"http://localhost:8080",
		"ftp://x",
		"https://*.example.com",
	}
	s.Normalize()
	want := []string{
		"https://shop.example.com",
		"http://localhost:8080",
		"ftp://x",
		"https://*.example.com",
	}
	assert.Equal(
		t,
		want,
		s.AllowedOrigins,
		"normalized, without blanks and duplicates",
	)
	codes := map[string]string{
		"allowedOrigins[2]": "invalid_origin",
		"allowedOrigins[3]": "invalid_origin",
	}
	assert.Equal(t, codes, fieldCodes(t, s.Validate(bases)))
}

func TestSiteRetention(t *testing.T) {
	for days, valid := range map[int]bool{
		0:     true,
		1:     true,
		36500: true,
		-1:    false,
		36501: false,
	} {
		s := validSite()
		s.IncidentRetentionDays = days
		s.Normalize()
		if valid {
			assert.NoError(t, s.Validate(bases), days)
		} else {
			want := map[string]string{"incidentRetentionDays": "out_of_range"}
			assert.Equal(t, want, fieldCodes(t, s.Validate(bases)), days)
		}
	}
}

func TestSiteBranding(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	s := validSite()
	s.BrandColor = " #A0B1C2 "
	s.Logo = "\n<!-- logo --><svg xmlns=\"http://www.w3.org/2000/svg\" onload=\"x()\"><script/><rect/></svg>\n"
	s.Normalize()
	require.NoError(s.Validate(bases))
	assert.Equal("#a0b1c2", s.BrandColor)
	want := `<svg xmlns="http://www.w3.org/2000/svg"><rect></rect></svg>`
	assert.Equal(want, s.Logo, "sanitized on save")

	s.Logo = "<svg xmlns=\"http://www.w3.org/2000/svg\"><path d=\"" +
		strings.Repeat("M", 70000) + "\"/></svg>"
	s.Normalize()
	codes := map[string]string{"logo": "svg_too_large"}
	assert.Equal(codes, fieldCodes(t, s.Validate(bases)))

	s.Logo = "  "
	s.Normalize()
	assert.Empty(s.Logo)
}

func TestSiteAvailability(t *testing.T) {
	for availability, online := range map[string]bool{
		AvailabilityOnline:  true,
		AvailabilityOffline: false,
		AvailabilityPaused:  false,
	} {
		s := &Site{Availability: availability}
		assert.Equal(t, online, s.Online(), availability)
	}
}

package model

import (
	"slices"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/i18n"
)

// Themes an admin can choose as default color scheme.
var Themes = []string{"light", "dark", "system"}

// Settings are the instance-wide settings.
type Settings struct {
	Languages    Languages `json:"languages"`
	DefaultTheme string    `json:"defaultTheme"`
	// Legal are the legal pages of the landing page, which sites inherit.
	Legal Legal `json:"legal"`
	// Landing is the Markdown text of the landing page.
	Landing Text `json:"landing,omitempty"`
}

// DefaultSettings returns the settings of a fresh instance: every supported
// language enabled, the reference language first and primary.
func DefaultSettings() Settings {
	langs := i18n.Supported()
	i := slices.Index(langs, i18n.Reference)
	langs = append([]string{i18n.Reference}, slices.Delete(langs, i, i+1)...)
	return Settings{
		Languages: Languages{
			Enabled: langs,
			Primary: i18n.Reference,
		},
		DefaultTheme: "system",
		Legal: Legal{
			Imprint: LegalPage{Mode: LegalNone},
			Privacy: LegalPage{Mode: LegalNone},
		},
	}
}

// Normalize trims the texts, fills in defaults and drops the fields the
// legal pages' modes do not use.
func (s *Settings) Normalize() {
	s.Legal = s.Legal.normalize(LegalNone)
	s.Landing = s.Landing.Normalize()
}

// Validate checks normalized settings.
func (s Settings) Validate() error {
	var f apierr.Fields
	s.Languages.validate(&f, "languages")

	if !slices.Contains(Themes, s.DefaultTheme) {
		f.Add("defaultTheme", apierr.InvalidValue)
	}

	s.Legal.validate(&f, "legal", s.Languages, legalModes)
	validateText(&f, "landing", s.Landing, s.Languages, false, maxMarkdown)
	return f.Err()
}

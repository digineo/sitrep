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
	}
}

// Validate checks the settings.
func (s Settings) Validate() error {
	var f apierr.Fields
	s.Languages.validate(&f, "languages")

	if !slices.Contains(Themes, s.DefaultTheme) {
		f.Add("defaultTheme", apierr.InvalidValue)
	}
	return f.Err()
}

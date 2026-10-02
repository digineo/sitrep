package model

import (
	"fmt"
	"slices"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/i18n"
)

// Languages are the enabled languages, in display order, and the primary
// one among them.
type Languages struct {
	Enabled []string `json:"enabled"`
	Primary string   `json:"primary"`
}

// Effective drops languages that have no catalog anymore. If the primary
// language was dropped, the first remaining one takes its place. If none
// remain, the reference language is used.
func (l Languages) Effective() Languages {
	enabled := slices.DeleteFunc(slices.Clone(l.Enabled), func(lang string) bool {
		return !i18n.IsSupported(lang)
	})
	if len(enabled) == 0 {
		return Languages{
			Enabled: []string{i18n.Reference},
			Primary: i18n.Reference,
		}
	}

	primary := l.Primary
	if !slices.Contains(enabled, primary) {
		primary = enabled[0]
	}
	return Languages{
		Enabled: enabled,
		Primary: primary,
	}
}

func (l Languages) validate(f *apierr.Fields, path string) {
	if len(l.Enabled) == 0 {
		f.Add(path+".enabled", apierr.Required)
	}

	for i, lang := range l.Enabled {
		field := fmt.Sprintf("%s.enabled[%d]", path, i)
		switch {
		case !i18n.IsSupported(lang):
			f.Add(field, apierr.UnsupportedLanguage)
		case slices.Index(l.Enabled, lang) < i:
			f.Add(field, apierr.Duplicate)
		}
	}

	switch {
	case l.Primary == "":
		f.Add(path+".primary", apierr.Required)
	case !slices.Contains(l.Enabled, l.Primary):
		f.Add(path+".primary", apierr.PrimaryNotEnabled)
	}
}

// Text is a localized text: one string per language code. Values are
// trimmed on save, so blank values are empty.
type Text map[string]string

// Resolve returns the text to show in lang. It falls back to the primary
// language, then to the first enabled language that has a value.
func (t Text) Resolve(lang string, l Languages) string {
	if v := t[lang]; v != "" {
		return v
	}

	if v := t[l.Primary]; v != "" {
		return v
	}

	for _, lang := range l.Enabled {
		if v := t[lang]; v != "" {
			return v
		}
	}
	return ""
}

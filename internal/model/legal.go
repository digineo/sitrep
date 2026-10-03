package model

import (
	"cmp"
	"maps"
	"net/url"
	"slices"

	"github.com/digineo/sitrep/internal/apierr"
)

// Modes of a legal page. Only sites can inherit the instance's page.
const (
	LegalInherit = "inherit"
	LegalNone    = "none"
	LegalURL     = "url"
	LegalText    = "text"
)

const maxURL = 2000

var (
	legalModes     = []string{LegalNone, LegalURL, LegalText}
	siteLegalModes = []string{LegalInherit, LegalNone, LegalURL, LegalText}
)

// Legal holds the imprint and the privacy statement.
type Legal struct {
	Imprint LegalPage `json:"imprint"`
	Privacy LegalPage `json:"privacy"`
}

// LegalPage is a legal page: none, a link to a page elsewhere, or a
// Markdown text.
type LegalPage struct {
	Mode string `json:"mode"`
	URL  Text   `json:"url,omitempty"`
	Text Text   `json:"text,omitempty"`
}

// Inherits reports whether a site shows the instance's page instead.
func (p LegalPage) Inherits() bool {
	return p.Mode != LegalNone && p.Mode != LegalURL && p.Mode != LegalText
}

func (l Legal) normalize(mode string) Legal {
	return Legal{
		Imprint: l.Imprint.normalize(mode),
		Privacy: l.Privacy.normalize(mode),
	}
}

// normalize fills in the default mode, trims the texts and drops the field
// the mode does not use.
func (p LegalPage) normalize(mode string) LegalPage {
	out := LegalPage{Mode: cmp.Or(p.Mode, mode)}
	switch out.Mode {
	case LegalURL:
		out.URL = p.URL.Normalize()
	case LegalText:
		out.Text = p.Text.Normalize()
	}
	return out
}

func (l Legal) validate(
	f *apierr.Fields,
	path string,
	langs Languages,
	modes []string,
) {
	l.Imprint.validate(f, path+".imprint", langs, modes)
	l.Privacy.validate(f, path+".privacy", langs, modes)
}

func (p LegalPage) validate(
	f *apierr.Fields,
	path string,
	l Languages,
	modes []string,
) {
	if !slices.Contains(modes, p.Mode) {
		f.Add(path+".mode", apierr.InvalidValue)
		return
	}

	validateText(f, path+".url", p.URL, l, p.Mode == LegalURL, maxURL)
	for _, lang := range slices.Sorted(maps.Keys(p.URL)) {
		if !validURL(p.URL[lang]) {
			f.Add(path+".url."+lang, apierr.InvalidLink)
		}
	}

	validateText(f, path+".text", p.Text, l, p.Mode == LegalText, maxMarkdown)
}

// Texts returns the localized texts of the pages.
func (l Legal) Texts() []Text {
	return []Text{l.Imprint.URL, l.Imprint.Text, l.Privacy.URL, l.Privacy.Text}
}

// validURL reports whether s is an absolute http or https URL.
func validURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

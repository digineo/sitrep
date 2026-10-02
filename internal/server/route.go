package server

import (
	"slices"
	"strings"

	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
)

type pageKind int

const (
	pageNotFound pageKind = iota
	pageOverview
	pageArchive
	pageIncident
	pageImprint
	pagePrivacy
	pageFeed
	pageJSON
)

type page struct {
	kind pageKind
	id   string // incident ID
}

// resolved is the outcome of routing a path below a base.
type resolved struct {
	page       page
	lang       string
	redirect   string // canonical path below the base, if the path is not canonical
	negotiated bool   // whether the redirect depends on language negotiation
}

// routePage resolves a path below a base to a page in one of the enabled
// languages. Landing pages (landing=true) are the overview and the legal
// pages. Non-canonical paths resolve to a redirect to the canonical one.
// negotiate returns the visitor's preferred enabled language.
func routePage(
	rest string,
	langs model.Languages,
	landing bool,
	negotiate func() string,
) resolved {
	if rest == "" {
		return resolved{
			page:     page{kind: pageOverview},
			lang:     negotiate(),
			redirect: "/",
		}
	}

	path := strings.TrimPrefix(rest, "/")
	var prefix string
	bare := false
	if seg, after, found := strings.Cut(path, "/"); i18n.ValidCode(seg) {
		if !i18n.IsSupported(seg) {
			return resolved{lang: negotiate()}
		}
		prefix = seg
		path = after
		bare = !found
	}

	p, slugLangs := parsePage(path, landing)
	multi := len(langs.Enabled) > 1
	enabled := slices.Contains(langs.Enabled, prefix)

	switch {
	case p.kind == pageNotFound && multi && enabled:
		return resolved{lang: prefix}
	case p.kind == pageNotFound:
		return resolved{lang: negotiate()}

	case !multi:
		lang := langs.Enabled[0]
		if prefix != "" || (slugLangs != nil && !slices.Contains(slugLangs, lang)) {
			return resolved{
				page:     p,
				lang:     lang,
				redirect: pagePath(p, lang, false),
			}
		}
		return resolved{
			page: p,
			lang: lang,
		}

	case prefix == "":
		for _, lang := range langs.Enabled {
			if slices.Contains(slugLangs, lang) {
				return resolved{
					page:     p,
					lang:     lang,
					redirect: pagePath(p, lang, true),
				}
			}
		}
		fallthrough
	case !enabled:
		lang := negotiate()
		return resolved{
			page:       p,
			lang:       lang,
			redirect:   pagePath(p, lang, true),
			negotiated: true,
		}

	case bare || (slugLangs != nil && !slices.Contains(slugLangs, prefix)):
		return resolved{
			page:     p,
			lang:     prefix,
			redirect: pagePath(p, prefix, true),
		}
	}
	return resolved{
		page: p,
		lang: prefix,
	}
}

// parsePage parses a path after the language segment, without leading
// slash. For legal pages, it also returns the languages the slug belongs to.
func parsePage(path string, landing bool) (page, []string) {
	if path == "" {
		return page{kind: pageOverview}, nil
	}

	if !landing {
		switch path {
		case "incidents":
			return page{kind: pageArchive}, nil
		case "feed.atom":
			return page{kind: pageFeed}, nil
		case "incidents.json":
			return page{kind: pageJSON}, nil
		}

		id, ok := strings.CutPrefix(path, "incidents/")
		if ok && id != "" && !strings.Contains(id, "/") {
			p := page{
				kind: pageIncident,
				id:   id,
			}
			return p, nil
		}
	}

	var kind pageKind
	var langs []string
	for _, lang := range i18n.Supported() {
		switch c := i18n.Get(lang); path {
		case c.Imprint:
			kind = pageImprint
			langs = append(langs, lang)
		case c.Privacy:
			kind = pagePrivacy
			langs = append(langs, lang)
		}
	}
	return page{kind: kind}, langs
}

// pagePath returns the path of p in lang below the base, with or without
// the language segment.
func pagePath(p page, lang string, prefixed bool) string {
	var s string
	switch p.kind {
	case pageOverview:
		s = "/"
	case pageArchive:
		s = "/incidents"
	case pageIncident:
		s = "/incidents/" + p.id
	case pageImprint:
		s = "/" + i18n.Get(lang).Imprint
	case pagePrivacy:
		s = "/" + i18n.Get(lang).Privacy
	case pageFeed:
		s = "/feed.atom"
	case pageJSON:
		s = "/incidents.json"
	}

	if prefixed {
		s = "/" + lang + s
	}
	return s
}

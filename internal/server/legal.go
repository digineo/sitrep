package server

import (
	"net/http"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/markdown"
	"github.com/digineo/sitrep/internal/model"
)

// legalKinds maps the names of legal pages in the public API to their
// pages.
var legalKinds = map[string]pageKind{
	"imprint": pageImprint,
	"privacy": pagePrivacy,
}

// effectiveLegal returns a site's legal page of kind, or the instance's
// if the site inherits it or site is nil, with the languages of its owner.
func effectiveLegal(
	kind pageKind,
	site *model.Site,
	settings model.Settings,
) (model.LegalPage, model.Languages) {
	pick := func(l model.Legal) model.LegalPage {
		if kind == pageImprint {
			return l.Imprint
		}
		return l.Privacy
	}

	if site != nil {
		if p := pick(site.Legal); !p.Inherits() {
			return p, site.Languages.Effective()
		}
	}
	return pick(settings.Legal), settings.Languages.Effective()
}

// legalTitle returns the name of a legal page in c's language.
func legalTitle(c *i18n.Catalog, kind pageKind) string {
	if kind == pageImprint {
		return c.T("legal.imprint.title", nil)
	}
	return c.T("legal.privacy.title", nil)
}

// legalLink is a legal page in a footer: a text page at its own path, or a
// link to a page elsewhere.
type legalLink struct {
	Mode string `json:"mode"` // url or text
	URL  string `json:"url,omitempty"`
}

// legalLinks are the legal pages to link; pages in mode none are left out.
type legalLinks struct {
	Imprint *legalLink `json:"imprint,omitempty"`
	Privacy *legalLink `json:"privacy,omitempty"`
}

// newLegalLinks returns the legal pages of a site, or of the instance if
// site is nil, in lang.
func newLegalLinks(
	site *model.Site,
	settings model.Settings,
	lang string,
) legalLinks {
	link := func(kind pageKind) *legalLink {
		switch p, langs := effectiveLegal(kind, site, settings); p.Mode {
		case model.LegalURL:
			return &legalLink{
				Mode: p.Mode,
				URL:  p.URL.Resolve(lang, langs),
			}
		case model.LegalText:
			return &legalLink{Mode: p.Mode}
		}
		return nil
	}
	return legalLinks{
		Imprint: link(pageImprint),
		Privacy: link(pagePrivacy),
	}
}

// publicLegal answers the HTML of a legal page in text mode, of a site or,
// without site, of the instance.
func (s *Server) publicLegal(w http.ResponseWriter, r *http.Request) {
	kind, ok := legalKinds[r.PathValue("kind")]
	if !ok {
		e := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	settings, err := s.db.Settings()
	var site *model.Site
	if err == nil && r.PathValue("site") != "" {
		site, err = s.publicSiteOf(r)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	q := r.URL.Query()
	lang := contentLang(q.Get("lang"), settings.Languages.Effective())
	if site != nil {
		lang = contentLang(q.Get("lang"), site.Languages.Effective())
	}

	p, langs := effectiveLegal(kind, site, settings)
	if p.Mode != model.LegalText {
		e := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	html := markdown.HTML(p.Text.Resolve(lang, langs))
	s.writePublic(w, r, map[string]string{"html": html}, nil)
}

// publicLegalLinks answers the legal pages of the base domains to link in
// lang, the promoted site's or the instance's, for the landing page and the
// console's login screen.
func (s *Server) publicLegalLinks(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.Settings()
	var site *model.Site
	if err == nil && settings.LandingSite != "" {
		site, err = s.db.Site(settings.LandingSite)
	}

	lang := contentLang(r.URL.Query().Get("lang"), settings.Languages.Effective())
	s.writePublic(w, r, newLegalLinks(site, settings, lang), err)
}

// publicLanding answers the rendered landing text, empty if there is none.
func (s *Server) publicLanding(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.Settings()
	langs := settings.Languages.Effective()
	lang := contentLang(r.URL.Query().Get("lang"), langs)
	html := markdown.HTML(settings.Landing.Resolve(lang, langs))
	s.writePublic(w, r, map[string]string{"html": html}, err)
}

// legalShell resolves a legal page for its shell. A page in url mode is
// answered with a redirect to its URL, and handled is true. A page in text
// mode returns its title; a page in mode none returns "".
func legalShell(
	w http.ResponseWriter,
	r *http.Request,
	res resolved,
	site *model.Site,
	settings model.Settings,
) (title string, handled bool) {
	switch p, langs := effectiveLegal(res.page.kind, site, settings); p.Mode {
	case model.LegalURL:
		http.Redirect(w, r, p.URL.Resolve(res.lang, langs), http.StatusFound)
		return "", true
	case model.LegalText:
		return legalTitle(i18n.Get(res.lang), res.page.kind), false
	}
	return "", false
}

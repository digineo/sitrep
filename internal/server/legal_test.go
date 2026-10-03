package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/model"
)

// putLegal sets the instance's legal pages and landing text.
func (f *fixture) putLegal(langs []string, legal obj, landing obj) {
	f.t.Helper()
	settings := obj{
		"languages": obj{
			"enabled": langs,
			"primary": langs[0],
		},
		"defaultTheme": "system",
		"legal":        legal,
		"landing":      landing,
	}
	w := f.admin(http.MethodPut, "/api/admin/settings", settings)
	require.Equal(f.t, http.StatusOK, w.Code)
}

// putSiteLegal replaces the legal pages of a site.
func (f *fixture) putSiteLegal(id string, legal model.Legal) {
	f.t.Helper()
	get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](f.t, get, http.StatusOK)
	site.Legal = legal
	put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	require.Equal(f.t, http.StatusOK, put.Code)
}

var instanceLegal = obj{
	"imprint": obj{
		"mode": "text",
		"text": obj{
			"en": "Instance **imprint**",
			"de": "Impressum der Instanz",
		},
	},
	"privacy": obj{
		"mode": "url",
		"url": obj{
			"en": "https://example.com/privacy",
			"de": "https://example.com/datenschutz",
		},
	},
}

func TestLegalShells(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	f.putLegal([]string{"en", "de"}, instanceLegal, nil)
	shop := f.createSite("shop", "en", "de")

	w := f.get("status.example.com", "/shop/de/impressum")
	require.Equal(http.StatusOK, w.Code)
	assert.Contains(
		w.Body.String(),
		"<title>Impressum · Site de</title>",
		"inherited",
	)
	assert.Contains(
		w.Body.String(),
		`<link rel="canonical" href="http://status.example.com/shop/de/impressum">`,
	)
	w = f.get("status.example.com", "/shop/de/datenschutz")
	assert.Equal(http.StatusFound, w.Code)
	assert.Equal(
		"https://example.com/datenschutz",
		w.Header().Get("Location"),
		"the link in the page's language",
	)
	assert.Equal(
		"https://example.com/privacy",
		f.get("status.example.com", "/shop/en/privacy").Header().Get("Location"),
	)

	f.putSiteLegal(shop, model.Legal{
		Imprint: model.LegalPage{
			Mode: model.LegalURL,
			URL:  model.Text{"en": "https://shop.example/imprint"},
		},
		Privacy: model.LegalPage{Mode: model.LegalNone},
	})
	w = f.get("status.example.com", "/shop/de/impressum")
	assert.Equal(
		"https://shop.example/imprint",
		w.Header().Get("Location"),
		"falls back to the primary language",
	)
	w = f.get("status.example.com", "/shop/en/privacy")
	assert.Equal(http.StatusNotFound, w.Code)
	assert.Contains(w.Body.String(), "<title>Page not found · Site en</title>")

	w = f.get("status.example.com", "/de/impressum")
	require.Equal(http.StatusOK, w.Code)
	assert.Contains(w.Body.String(), "<title>Impressum · Statusseiten</title>")
	assert.Equal("landing", parseBootstrap(t, w.Body.String()).Mode)
	assert.Equal(
		"https://example.com/privacy",
		f.get("status.example.com", "/en/privacy").Header().Get("Location"),
	)

	legal := obj{
		"imprint": obj{"mode": "none"},
		"privacy": obj{"mode": "none"},
	}
	f.putLegal([]string{"en"}, legal, nil)
	assert.Equal(http.StatusNotFound, f.get("status.example.com", "/imprint").Code)
	assert.Equal(
		http.StatusFound,
		f.get("status.example.com", "/impressum").Code,
		"slugs of other languages still redirect",
	)
}

func TestPublicLegal(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t)
	f.putLegal([]string{"en", "de"}, instanceLegal, obj{
		"en": "Welcome to *Acme*",
		"de": "Willkommen",
	})
	shop := f.createSite("shop", "en", "de")

	type page struct {
		HTML string `json:"html"`
	}
	get := func(path string) page {
		t.Helper()
		w := f.get("status.example.com", path)
		assert.Equal("public, max-age=10", w.Header().Get("Cache-Control"))
		return decode[page](t, w, http.StatusOK)
	}

	assert.Equal(
		"<p>Impressum der Instanz</p>\n",
		get("/api/public/sites/"+shop+"/legal/imprint?lang=de").HTML,
	)
	assert.Equal(
		"<p>Instance <strong>imprint</strong></p>\n",
		get("/api/public/legal/imprint?lang=en").HTML,
	)
	assert.Equal(
		"<p>Instance <strong>imprint</strong></p>\n",
		get("/api/public/legal/imprint?lang=fr").HTML,
	)
	for _, path := range []string{
		"/api/public/legal/privacy",
		"/api/public/legal/terms",
		"/api/public/sites/" + shop + "/legal/privacy",
	} {
		assert.Equal(
			http.StatusNotFound,
			f.get("status.example.com", path).Code,
			path,
		)
	}

	assert.Equal("<p>Willkommen</p>\n", get("/api/public/landing?lang=de").HTML)
	want := legalLinks{
		Imprint: &legalLink{Mode: "text"},
		Privacy: &legalLink{
			Mode: "url",
			URL:  "https://example.com/datenschutz",
		},
	}
	w := f.get("status.example.com", "/api/public/legal?lang=de")
	assert.Equal(want, decode[legalLinks](t, w, http.StatusOK))

	p, _ := f.publicPayload(shop, "?lang=de")
	assert.Equal(want, p.Site.Legal)

	// German is stored for the instance, but not enabled there, so a
	// German site shows the English page it inherits.
	f.putLegal([]string{"en"}, instanceLegal, nil)
	berlin := f.createSite("berlin", "de")
	assert.Equal(
		"<p>Instance <strong>imprint</strong></p>\n",
		get("/api/public/sites/"+berlin+"/legal/imprint?lang=de").HTML,
	)
	assert.Empty(get("/api/public/landing?lang=de").HTML, "no landing text")
}

func TestLegalMissingTranslations(t *testing.T) {
	f := newFixture(t)
	id := f.createSite("shop", "en", "de")
	get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](t, get, http.StatusOK)
	site.Name = model.Text{
		"en": "Shop",
		"de": "Laden",
	}
	site.Legal.Imprint = model.LegalPage{
		Mode: model.LegalText,
		Text: model.Text{"en": "Imprint"},
	}
	put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	require.Equal(t, http.StatusOK, put.Code)
	w := f.admin(http.MethodGet, "/api/admin/sites", nil)
	for _, sum := range decode[[]siteSummary](t, w, http.StatusOK) {
		if sum.ID == id {
			assert.Equal(t, 1, sum.Missing, "the imprint lacks German")
		}
	}
}

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/digineo/sitrep/internal/model"
)

func TestRoutePage(t *testing.T) {
	single := model.Languages{
		Enabled: []string{"de"},
		Primary: "de",
	}
	multi := model.Languages{
		Enabled: []string{"de", "en"},
		Primary: "de",
	}
	// "xx" has no catalog: it lets "de" be a supported language that is not
	// enabled. Paths never resolve to legal pages in "xx".
	partial := model.Languages{
		Enabled: []string{"en", "xx"},
		Primary: "en",
	}
	type want = resolved
	overview := page{kind: pageOverview}
	archive := page{kind: pageArchive}
	imprint := page{kind: pageImprint}
	privacy := page{kind: pagePrivacy}
	tests := []struct {
		langs model.Languages
		path  string
		want  resolved
	}{
		// one language: canonical URLs carry no language segment
		{single, "/", want{page: overview, lang: "de"}},
		{single, "/incidents", want{page: archive, lang: "de"}},
		{single, "/incidents/0192", want{page: page{kind: pageIncident, id: "0192"}, lang: "de"}},
		{single, "/feed.atom", want{page: page{kind: pageFeed}, lang: "de"}},
		{single, "/incidents.json", want{page: page{kind: pageJSON}, lang: "de"}},
		{single, "/impressum", want{page: imprint, lang: "de"}},
		{single, "/datenschutz", want{page: privacy, lang: "de"}},
		{single, "/imprint", want{page: imprint, lang: "de", redirect: "/impressum"}},
		{single, "/privacy", want{page: privacy, lang: "de", redirect: "/datenschutz"}},
		{single, "/de/", want{page: overview, lang: "de", redirect: "/"}},
		{single, "/de", want{page: overview, lang: "de", redirect: "/"}},
		{single, "/en/", want{page: overview, lang: "de", redirect: "/"}},
		{single, "/en/incidents", want{page: archive, lang: "de", redirect: "/incidents"}},
		{single, "/en/incidents/0192", want{page: page{kind: pageIncident, id: "0192"}, lang: "de", redirect: "/incidents/0192"}},
		{single, "/en/imprint", want{page: imprint, lang: "de", redirect: "/impressum"}},
		{single, "", want{page: overview, lang: "de", redirect: "/"}},
		{single, "/xx/", want{lang: "de"}},
		{single, "/unknown", want{lang: "de"}},
		{single, "/incidents/", want{lang: "de"}},
		{single, "/incidents/a/b", want{lang: "de"}},
		{single, "/Incidents", want{lang: "de"}},
		{single, "/feed.atom/", want{lang: "de"}},
		{single, "/en/unknown", want{lang: "de"}},

		// several languages: canonical URLs carry the language
		{multi, "/de/", want{page: overview, lang: "de"}},
		{multi, "/en/incidents", want{page: archive, lang: "en"}},
		{multi, "/de/impressum", want{page: imprint, lang: "de"}},
		{multi, "/en/privacy", want{page: privacy, lang: "en"}},
		{multi, "/", want{page: overview, lang: "en", redirect: "/en/", negotiated: true}},
		{multi, "/incidents", want{page: archive, lang: "en", redirect: "/en/incidents", negotiated: true}},
		{multi, "/incidents/0192", want{page: page{kind: pageIncident, id: "0192"}, lang: "en", redirect: "/en/incidents/0192", negotiated: true}},
		{multi, "/feed.atom", want{page: page{kind: pageFeed}, lang: "en", redirect: "/en/feed.atom", negotiated: true}},
		{multi, "/incidents.json", want{page: page{kind: pageJSON}, lang: "en", redirect: "/en/incidents.json", negotiated: true}},
		{multi, "/de", want{page: overview, lang: "de", redirect: "/de/"}},
		{multi, "/impressum", want{page: imprint, lang: "de", redirect: "/de/impressum"}},
		{multi, "/privacy", want{page: privacy, lang: "en", redirect: "/en/privacy"}},
		{multi, "/de/privacy", want{page: privacy, lang: "de", redirect: "/de/datenschutz"}},
		{multi, "/en/impressum", want{page: imprint, lang: "en", redirect: "/en/imprint"}},
		{multi, "/de/unknown", want{lang: "de"}},
		{multi, "/unknown", want{lang: "en"}},
		{multi, "/xx/", want{lang: "en"}},

		// several languages, one supported language not enabled
		{partial, "/de/", want{page: overview, lang: "en", redirect: "/en/", negotiated: true}},
		{partial, "/de", want{page: overview, lang: "en", redirect: "/en/", negotiated: true}},
		{partial, "/de/incidents", want{page: archive, lang: "en", redirect: "/en/incidents", negotiated: true}},
		{partial, "/de/impressum", want{page: imprint, lang: "en", redirect: "/en/imprint", negotiated: true}},
		{partial, "/impressum", want{page: imprint, lang: "en", redirect: "/en/imprint", negotiated: true}},
	}
	for _, tt := range tests {
		// negotiation always yields an enabled language
		negotiate := func() string { return "en" }
		if len(tt.langs.Enabled) == 1 {
			negotiate = func() string { return "de" }
		}

		got := routePage(tt.path, tt.langs, false, negotiate)
		assert.Equal(t, tt.want, got, "%v %s", tt.langs.Enabled, tt.path)
	}
}

func TestRouteLandingPage(t *testing.T) {
	langs := model.Languages{
		Enabled: []string{"en", "de"},
		Primary: "en",
	}
	negotiate := func() string { return "de" }
	want := resolved{
		page:       page{kind: pageOverview},
		lang:       "de",
		redirect:   "/de/",
		negotiated: true,
	}
	assert.Equal(t, want, routePage("/", langs, true, negotiate))

	want = resolved{
		page: page{kind: pageImprint},
		lang: "de",
	}
	assert.Equal(t, want, routePage("/de/impressum", langs, true, negotiate))

	for _, path := range []string{
		"/incidents",
		"/de/incidents",
		"/de/feed.atom",
		"/de/incidents.json",
		"/de/incidents/0192",
	} {
		kind := routePage(path, langs, true, negotiate).page.kind
		assert.Equal(t, pageNotFound, kind, path)
	}
}

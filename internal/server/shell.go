package server

import (
	"encoding/base64"
	"html/template"
	"net/http"
	"slices"

	"github.com/digineo/sitrep/frontend"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
)

// shellAssets are the URLs a shell links for one entry point.
type shellAssets struct {
	Scripts []string
	Styles  []string
}

// icon is the product logo as favicon. The public pages replace it with the
// status favicon once they know the status.
var icon = template.URL(
	"data:image/svg+xml;base64," +
		base64.StdEncoding.EncodeToString(frontend.Logo),
)

// The bootstrap data is rendered in a JSON script element, where
// html/template escapes it so that it cannot end the element.
var shellTemplate = template.Must(template.New("shell").Parse(`<!doctype html>
<html lang="{{.Bootstrap.Lang}}"{{with .Theme}} data-theme="{{.}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="icon" type="image/svg+xml" href="{{.Icon}}">
{{- with .Canonical}}
<link rel="canonical" href="{{.}}">
{{- end}}
{{- range .Alternates}}
<link rel="alternate" hreflang="{{.Lang}}" href="{{.Href}}">
{{- end}}
{{- range .Assets.Styles}}
<link rel="stylesheet" href="{{.}}">
{{- end}}
{{- range .Assets.Scripts}}
<script type="module" src="{{.}}"></script>
{{- end}}
<script type="application/json" id="bootstrap">{{.Bootstrap}}</script>
</head>
<body>
<div id="app"></div>
</body>
</html>
`))

type shell struct {
	Icon       template.URL
	Theme      string
	Title      string
	Canonical  string
	Alternates []alternate
	Assets     shellAssets
	Bootstrap  bootstrap
}

type alternate struct {
	Lang string
	Href string
}

// bootstrap tells the single-page application what to render. The admin
// console also learns the base domains, for route hints, and the default
// refresh interval of panels.
type bootstrap struct {
	Mode           string   `json:"mode"` // site, landing or admin
	SiteID         string   `json:"siteId,omitempty"`
	BasePath       string   `json:"basePath"`
	Lang           string   `json:"lang"`
	Languages      []string `json:"languages"`
	Primary        string   `json:"primary"`
	BaseDomains    []string `json:"baseDomains,omitempty"`
	DefaultRefresh string   `json:"defaultRefresh,omitempty"`
}

const (
	cspPublic = "default-src 'self'; " +
		"img-src 'self' data:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'self'"
	cspAdmin = cspPublic + "; frame-ancestors 'none'"
)

func writeShell(w http.ResponseWriter, status int, sh shell) {
	csp := cspPublic
	if sh.Bootstrap.Mode == "admin" {
		csp = cspAdmin
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", csp)
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.WriteHeader(status)
	sh.Icon = icon
	_ = shellTemplate.Execute(w, sh)
}

// theme returns the color scheme to stamp for the first paint: the visitor's
// choice, else the default. "system" stamps nothing.
func theme(r *http.Request, def string) string {
	c, err := r.Cookie("theme")
	if err == nil && slices.Contains(model.Themes, c.Value) {
		def = c.Value
	}
	if def == "system" {
		return ""
	}
	return def
}

// pageShell returns the shell of a site page or landing page. base is the
// site's base path; title is the view title, or "" for the overview.
func pageShell(
	info httpx.Info,
	base string,
	langs model.Languages,
	res resolved,
	title, name string,
) shell {
	sh := shell{
		Title: name,
		Bootstrap: bootstrap{
			BasePath:  base,
			Lang:      res.lang,
			Languages: langs.Enabled,
			Primary:   langs.Primary,
		},
	}
	if title != "" {
		sh.Title = i18n.Get(res.lang).T("page.title", map[string]string{
			"view": title,
			"site": name,
		})
	}

	if res.page.kind == pageNotFound {
		return sh
	}

	multi := len(langs.Enabled) > 1
	sh.Canonical = info.Origin + base + pagePath(res.page, res.lang, multi)
	if multi {
		for _, lang := range langs.Enabled {
			href := info.Origin + base + pagePath(res.page, lang, true)
			sh.Alternates = append(sh.Alternates, alternate{lang, href})
		}

		href := info.Origin + base + pagePath(res.page, res.lang, false)
		sh.Alternates = append(sh.Alternates, alternate{"x-default", href})
	}
	return sh
}

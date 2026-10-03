package server

import (
	"cmp"
	"crypto/sha1" // UUIDv5 is defined with SHA-1.
	"encoding/xml"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/markdown"
	"github.com/digineo/sitrep/internal/model"
)

const (
	maxFeedEntries   = 100
	maxFeedPrevious  = 3
	maxExcerptLength = 200
)

// uuidV5 returns the name-based UUID of name in the namespace ns.
func uuidV5(ns uuid.UUID, name string) uuid.UUID {
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	var u uuid.UUID
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x50
	u[8] = u[8]&0x3f | 0x80
	return u
}

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Lang    string      `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated time.Time   `xml:"updated"`
	Author  string      `xml:"author>name"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	ID        string      `xml:"id"`
	Title     string      `xml:"title"`
	Published time.Time   `xml:"published"`
	Updated   time.Time   `xml:"updated"`
	Link      atomLink    `xml:"link"`
	Content   atomContent `xml:"content"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

var feedContent = template.Must(template.New("content").Parse(`{{.Description}}
{{- with .Previous}}
<p>{{$.PreviousLabel}}</p>
<ul>
{{- range .}}
<li><time datetime="{{.At}}">{{.Time}}</time> {{.Label}}{{with .Excerpt}}: {{.}}{{end}}</li>
{{- end}}
</ul>
{{- end}}
<p><a href="{{.Link}}">{{.ViewLabel}}</a></p>
`))

type feedPrevious struct {
	At, Time, Label, Excerpt string
}

// updateLabel names what an update set: its status, or else its severity.
func updateLabel(c *i18n.Catalog, u model.Update) string {
	if u.Status != "" {
		return c.T("incidents.status."+u.Status, nil)
	}
	label := c.T("incidents.severity."+u.Severity, nil)
	return c.T("incidents.severityLabel", map[string]string{"label": label})
}

// formatTime formats t in the zone with the catalog's pattern and the
// numeric UTC offset, which needs no translation unlike zone names.
func formatTime(c *i18n.Catalog, t time.Time, loc *time.Location) string {
	t = t.In(loc)
	two := func(n int) string { return strconv.Itoa(100 + n)[1:] }
	return c.T("format.dateTime", map[string]string{
		"year":   strconv.Itoa(t.Year()),
		"month":  two(int(t.Month())),
		"day":    two(t.Day()),
		"hour":   two(t.Hour()),
		"minute": two(t.Minute()),
		"offset": t.Format("-07:00"),
	})
}

// serveFeed answers the Atom feed of a site's public incidents in lang:
// one entry per update, newest first.
func (s *Server) serveFeed(
	w http.ResponseWriter,
	r *http.Request,
	info httpx.Info,
	site *model.Site,
	base, lang string,
) {
	incidents, err := s.db.Incidents(site.ID)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := site.Languages.Effective()
	multi := len(langs.Enabled) > 1
	c := i18n.Get(lang)
	loc, _ := time.LoadLocation(site.Timezone)
	now := time.Now()
	name := site.Name.Resolve(lang, langs)
	url := func(p page) string {
		return info.Origin + base + pagePath(p, lang, multi)
	}

	type item struct {
		inc *model.Incident
		i   int
	}
	var items []item
	for _, inc := range incidents {
		if inc.Public(now) {
			for i := range inc.Updates {
				items = append(items, item{&inc, i})
			}
		}
	}

	slices.SortStableFunc(items, func(a, b item) int {
		ua, ub := a.inc.Updates[a.i], b.inc.Updates[b.i]
		return cmp.Or(ub.At.Compare(ua.At), cmp.Compare(ub.ID, ua.ID))
	})

	items = items[:min(len(items), maxFeedEntries)]

	feed := atomFeed{
		Lang:    lang,
		ID:      "urn:uuid:" + uuidV5(uuid.MustParse(site.ID), lang).String(),
		Title:   name,
		Updated: site.UpdatedAt,
		Author:  name,
		Links: []atomLink{
			{
				Rel:  "self",
				Type: "application/atom+xml",
				Href: url(page{kind: pageFeed}),
			},
			{
				Rel:  "alternate",
				Type: "text/html",
				Href: url(page{kind: pageOverview}),
			},
		},
	}
	if len(items) > 0 {
		feed.Updated = items[0].inc.Updates[items[0].i].At
	}

	for _, it := range items {
		u := it.inc.Updates[it.i]
		link := url(page{
			kind: pageIncident,
			id:   it.inc.ID,
		})
		desc := u.Description.Resolve(lang, langs)
		data := struct {
			Description              template.HTML
			Previous                 []feedPrevious
			PreviousLabel, ViewLabel string
			Link                     string
		}{
			Description:   template.HTML(markdown.HTML(desc)),
			PreviousLabel: c.T("feed.previous", nil),
			ViewLabel:     c.T("feed.view", nil),
			Link:          link,
		}
		for j := it.i - 1; j >= max(0, it.i-maxFeedPrevious); j-- {
			p := it.inc.Updates[j]
			text := p.Description.Resolve(lang, langs)
			data.Previous = append(data.Previous, feedPrevious{
				At:      p.At.UTC().Format(time.RFC3339),
				Time:    formatTime(c, p.At, loc),
				Label:   updateLabel(c, p),
				Excerpt: markdown.Excerpt(text, maxExcerptLength),
			})
		}

		var content strings.Builder
		if err := feedContent.Execute(&content, data); err != nil {
			httpx.WriteError(w, r, s.log, err)
			return
		}

		feed.Entries = append(feed.Entries, atomEntry{
			ID:        "urn:uuid:" + u.ID,
			Title:     it.inc.Title.Resolve(lang, langs) + ": " + updateLabel(c, u),
			Published: u.At,
			Updated:   u.At,
			Link: atomLink{
				Rel:  "alternate",
				Type: "text/html",
				Href: link,
			},
			Content: atomContent{
				Type: "html",
				Body: content.String(),
			},
		})
	}

	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

// serveIncidentsJSON answers the public incidents of a site in lang. Sites
// listed in the site's allowed origins may read it cross-origin.
func (s *Server) serveIncidentsJSON(
	w http.ResponseWriter,
	r *http.Request,
	site *model.Site,
	lang string,
) {
	incidents, err := s.db.Incidents(site.ID)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	h := w.Header()
	h.Add("Vary", "Origin")
	origin, ok := model.NormalizeOrigin(r.Header.Get("Origin"))
	if ok && slices.Contains(site.AllowedOrigins, origin) {
		h.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	}

	langs := site.Languages.Effective()
	now := time.Now()
	byActivity(incidents)
	list := []publicIncident{}
	for _, inc := range incidents {
		if inc.Public(now) {
			list = append(list, newPublicIncident(&inc, lang, langs))
		}
	}

	h.Set("Cache-Control", "public, max-age=60")
	httpx.WriteJSON(w, http.StatusOK, list)
}

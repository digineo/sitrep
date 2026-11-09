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
<p><a href="{{.Link}}" target="_blank" rel="noopener">{{.ViewLabel}}</a></p>
`))

type feedPrevious struct {
	At, Time, Label, Excerpt string
}

var teaserContent = template.Must(template.New("teaser").Parse(`<p>{{.Excerpt}}</p>
<p><a href="{{.Link}}" target="_blank" rel="noopener">{{.Label}}</a></p>
`))

// postmortemID returns the ID of the incident's postmortem in feeds.
func postmortemID(incident string) string {
	return uuidV5(uuid.MustParse(incident), "postmortem").String()
}

// postmortemTeaser renders an excerpt of the postmortem src with a link to
// the incident page at link, for feeds that leave out the full text.
func postmortemTeaser(c *i18n.Catalog, src, link string) (string, error) {
	var b strings.Builder
	err := teaserContent.Execute(&b, map[string]string{
		"Excerpt": markdown.Excerpt(src, maxExcerptLength),
		"Link":    link,
		"Label":   c.T("feed.readPostmortem", nil),
	})
	return b.String(), err
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
// one entry per update and per postmortem, newest first.
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
		i   int // the update's index, or -1 for the postmortem
		id  string
		at  time.Time
	}
	var items []item
	for _, inc := range incidents {
		if !inc.Public(now) {
			continue
		}
		for i, u := range inc.Updates {
			items = append(items, item{&inc, i, u.ID, u.At})
		}
		if inc.PostmortemAt != nil && inc.Postmortem.Resolve(lang, langs) != "" {
			items = append(items, item{&inc, -1, postmortemID(inc.ID), *inc.PostmortemAt})
		}
	}

	slices.SortStableFunc(items, func(a, b item) int {
		return cmp.Or(b.at.Compare(a.at), cmp.Compare(b.id, a.id))
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
		feed.Updated = items[0].at
	}

	for _, it := range items {
		link := url(page{
			kind: pageIncident,
			id:   it.inc.ID,
		})
		entry := atomEntry{
			ID:        "urn:uuid:" + it.id,
			Published: it.at,
			Updated:   it.at,
			Link: atomLink{
				Rel:  "alternate",
				Type: "text/html",
				Href: link,
			},
			Content: atomContent{Type: "html"},
		}
		title := it.inc.Title.Resolve(lang, langs) + ": "

		if it.i < 0 {
			src := it.inc.Postmortem.Resolve(lang, langs)
			body, err := postmortemTeaser(c, src, link)
			if err != nil {
				httpx.WriteError(w, r, s.log, err)
				return
			}
			entry.Title = title + c.T("incidents.postmortem", nil)
			entry.Content.Body = body
			feed.Entries = append(feed.Entries, entry)
			continue
		}

		u := it.inc.Updates[it.i]
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

		entry.Title = title + updateLabel(c, u)
		entry.Content.Body = content.String()
		feed.Entries = append(feed.Entries, entry)
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

// serveIncidentsJSON answers the public incidents of a site in lang. A
// postmortem is listed as an update with an excerpt and a link to the
// incident. Sites listed in the site's allowed origins may read it
// cross-origin.
func (s *Server) serveIncidentsJSON(
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

	h := w.Header()
	h.Add("Vary", "Origin")
	origin, ok := model.NormalizeOrigin(r.Header.Get("Origin"))
	if ok && slices.Contains(site.AllowedOrigins, origin) {
		h.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	}

	langs := site.Languages.Effective()
	multi := len(langs.Enabled) > 1
	c := i18n.Get(lang)
	now := time.Now()
	byActivity(incidents)
	list := []publicIncident{}
	for _, inc := range incidents {
		if !inc.Public(now) {
			continue
		}

		p := newPublicIncident(&inc, lang, langs)
		if p.Postmortem != nil {
			link := info.Origin + base + pagePath(page{
				kind: pageIncident,
				id:   inc.ID,
			}, lang, multi)
			src := inc.Postmortem.Resolve(lang, langs)
			html, err := postmortemTeaser(c, src, link)
			if err != nil {
				httpx.WriteError(w, r, s.log, err)
				return
			}
			p.listPostmortem(html)
		}
		list = append(list, p)
	}

	h.Set("Cache-Control", "public, max-age=60")
	httpx.WriteJSON(w, http.StatusOK, list)
}

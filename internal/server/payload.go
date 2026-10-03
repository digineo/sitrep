package server

import (
	"net/http"
	"time"
	"uuid"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/markdown"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/process"
)

// payload is a site's data for its page. Responses to requests with a
// cursor only carry the sections that changed after it; status is always
// included.
type payload struct {
	Cursor    string                  `json:"cursor"`
	Status    string                  `json:"status"`
	Site      *sitePayload            `json:"site,omitempty"`
	Incidents *incidentsPayload       `json:"incidents,omitempty"`
	Panels    map[string]panelPayload `json:"panels,omitempty"`
}

type sitePayload struct {
	Name       string      `json:"name"`
	Theme      string      `json:"theme"`
	BrandColor string      `json:"brandColor,omitempty"`
	Logo       string      `json:"logo,omitempty"` // URL
	Legal      legalLinks  `json:"legal"`
	Languages  []string    `json:"languages"`
	Timezone   string      `json:"timezone"`
	Panels     []panelInfo `json:"panels"`
}

// panelInfo is a panel's presentation, in display order.
type panelInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Decimals    *int   `json:"decimals,omitempty"`
	Style       string `json:"style,omitempty"`
	MinZero     bool   `json:"minZero,omitempty"`
	Range       int64  `json:"range,omitempty"` // seconds
}

type panelPayload struct {
	State     string     `json:"state"` // pending, fresh or stale
	FetchedAt *time.Time `json:"fetchedAt,omitempty"`
	Data      any        `json:"data,omitempty"`
	// For admins only: the latest poll's error and the data's warnings.
	Error    string            `json:"error,omitempty"`
	Warnings []process.Warning `json:"warnings,omitempty"`
}

// incidentsPayload holds the public incidents by phase, most recent
// activity first, and the spans to shade in charts.
type incidentsPayload struct {
	Ongoing  []publicIncident `json:"ongoing"`
	Upcoming []publicIncident `json:"upcoming"`
	Finished []publicIncident `json:"finished"`
	Spans    []span           `json:"spans"`
}

// publicIncident is an incident as visitors see it, without authors and
// Markdown sources. Updates are ordered by time.
type publicIncident struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Phase        string         `json:"phase"`
	Status       string         `json:"status"`
	Severity     string         `json:"severity,omitempty"`
	LastActivity time.Time      `json:"lastActivity"`
	Updates      []publicUpdate `json:"updates"`
}

type publicUpdate struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Status   string    `json:"status,omitempty"`
	Severity string    `json:"severity,omitempty"`
	HTML     string    `json:"html"` // the rendered description
}

// span is the time range of an incident in charts; Until is nil while the
// incident is ongoing.
type span struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Severity string     `json:"severity,omitempty"`
	From     time.Time  `json:"from"`
	Until    *time.Time `json:"until,omitempty"`
}

func newPublicIncident(
	inc *model.Incident,
	lang string,
	l model.Languages,
) publicIncident {
	p := publicIncident{
		ID:           inc.ID,
		Title:        inc.Title.Resolve(lang, l),
		Phase:        inc.Phase(),
		Status:       inc.Status(),
		Severity:     inc.Severity(),
		LastActivity: inc.LastActivity(),
		Updates:      []publicUpdate{},
	}
	for _, u := range inc.Updates {
		p.Updates = append(p.Updates, publicUpdate{
			ID:       u.ID,
			At:       u.At,
			Status:   u.Status,
			Severity: u.Severity,
			HTML:     markdown.HTML(u.Description.Resolve(lang, l)),
		})
	}
	return p
}

// incidentsSection builds the incidents section at now. Spans are those
// overlapping the last window of the site's widest chart range. It also
// returns when the section next changes with time alone, or zero.
func incidentsSection(
	incidents []model.Incident,
	panels []model.Panel,
	lang string,
	l model.Languages,
	now time.Time,
) (*incidentsPayload, time.Time) {
	var window time.Duration
	for _, p := range panels {
		if p.Type == model.PanelTimeseries {
			window = max(window, model.Duration(p.Range))
		}
	}

	var next time.Time
	changesAt := func(t time.Time) {
		if t.After(now) && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}

	sec := &incidentsPayload{
		Ongoing:  []publicIncident{},
		Upcoming: []publicIncident{},
		Finished: []publicIncident{},
		Spans:    []span{},
	}
	byActivity(incidents)
	for _, inc := range incidents {
		if inc.Public(now) {
			p := newPublicIncident(&inc, lang, l)
			switch p.Phase {
			case model.PhaseOngoing:
				sec.Ongoing = append(sec.Ongoing, p)
			case model.PhaseUpcoming:
				sec.Upcoming = append(sec.Upcoming, p)
			default:
				sec.Finished = append(sec.Finished, p)
				changesAt(inc.LastActivity().Add(model.PublicAfterFinish))
			}
		}

		from, until, ok := inc.Span()
		if !ok || window == 0 {
			continue
		}

		changesAt(from)
		if !until.IsZero() {
			changesAt(until.Add(window))
		}

		if from.After(now) || !until.IsZero() && until.Before(now.Add(-window)) {
			continue
		}

		sp := span{
			ID:       inc.ID,
			Title:    inc.Title.Resolve(lang, l),
			Severity: inc.Severity(),
			From:     from,
		}
		if !until.IsZero() {
			sp.Until = &until
		}
		sec.Spans = append(sec.Spans, sp)
	}
	return sec, next
}

// sitePayload builds the payload of site in lang with the sections changed
// after the cursor since, or all of them. Admin payloads carry the panels'
// errors and warnings.
func (s *Server) sitePayload(
	site *model.Site,
	lang, since string,
	admin bool,
) (payload, error) {
	// The sequence is read first, so that changes made while building
	// the payload are sent again with the next request.
	seq := s.poller.Seq()
	after, incremental := s.poller.Since(since)

	settings, err := s.db.Settings()
	if err != nil {
		return payload{}, err
	}

	panels, err := s.db.Panels(site.ID)
	if err != nil {
		return payload{}, err
	}

	incidents, err := s.db.Incidents(site.ID)
	if err != nil {
		return payload{}, err
	}

	langs := site.Languages.Effective()
	lang = contentLang(lang, langs)
	now := time.Now()
	p := payload{
		Cursor: s.poller.Cursor(seq),
		Status: s.status(panels, incidents).Overall,
		Panels: map[string]panelPayload{},
	}
	if !incremental || s.poller.IncidentsSeq(site.ID, now) > after {
		var next time.Time
		p.Incidents, next = incidentsSection(incidents, panels, lang, langs, now)
		if !next.IsZero() {
			s.poller.ExpireIncidents(site.ID, next)
		}
	}

	if !incremental || s.poller.SiteSeq(site.ID) > after {
		p.Site = &sitePayload{
			Name:       site.Name.Resolve(lang, langs),
			Theme:      site.Theme,
			BrandColor: site.BrandColor,
			Logo:       logoURL(site),
			Legal:      newLegalLinks(site, settings, lang),
			Languages:  langs.Enabled,
			Timezone:   site.Timezone,
			Panels:     []panelInfo{},
		}
		if site.Theme == "inherit" {
			p.Site.Theme = settings.DefaultTheme
		}

		for _, panel := range panels {
			p.Site.Panels = append(p.Site.Panels, panelInfo{
				ID:          panel.ID,
				Type:        panel.Type,
				Title:       panel.Title.Resolve(lang, langs),
				Description: panel.Description.Resolve(lang, langs),
				Unit:        panel.Unit.Resolve(lang, langs),
				Decimals:    panel.Decimals,
				Style:       panel.Style,
				MinZero:     panel.MinZero,
				Range:       int64(model.Duration(panel.Range) / time.Second),
			})
		}
	}

	for _, panel := range panels {
		e, ok := s.poller.Entry(panel.ID)
		if incremental && e.Seq <= after {
			continue
		}

		pp := panelPayload{State: "pending"}
		if ok && !e.Pending() {
			pp.State = "fresh"
			if e.Stale {
				pp.State = "stale"
			}
			pp.FetchedAt = &e.FetchedAt
			pp.Data = panelData(panel, e.Data, lang, langs, !e.Stale)
		}

		if admin {
			pp.Error = e.Err
			pp.Warnings = e.Data.Warnings()
		}

		p.Panels[panel.ID] = pp
	}
	return p, nil
}

// validID reports whether id is a UUID in canonical form.
func validID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == id
}

// publicSiteOf returns the site of a public API request. Malformed IDs are
// a bad request.
func (s *Server) publicSiteOf(r *http.Request) (*model.Site, error) {
	id := r.PathValue("site")
	if !validID(id) {
		return nil, apierr.New(http.StatusBadRequest, apierr.InvalidValue)
	}
	return s.db.Site(id)
}

// writePublic answers a successful public API request, cacheable for a
// short time, or the error.
func (s *Server) writePublic(
	w http.ResponseWriter,
	r *http.Request,
	v any,
	err error,
) {
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=10")
	httpx.WriteJSON(w, http.StatusOK, v)
}

// publicSite answers the site payload for visitors, in the requested
// language.
func (s *Server) publicSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.publicSiteOf(r)
	var p payload
	if err == nil {
		q := r.URL.Query()
		p, err = s.sitePayload(site, q.Get("lang"), q.Get("since"), false)
	}
	s.writePublic(w, r, p, err)
}

// publicIncident answers an incident of a site for visitors, in the
// requested language. Every incident of the site has a detail page.
func (s *Server) publicIncident(w http.ResponseWriter, r *http.Request) {
	site, err := s.publicSiteOf(r)
	id := r.PathValue("incident")
	if err == nil && !validID(id) {
		err = apierr.New(http.StatusBadRequest, apierr.InvalidValue)
	}

	var inc *model.Incident
	if err == nil {
		inc, err = s.db.Incident(site.ID, id)
	}

	var p publicIncident
	if err == nil {
		langs := site.Languages.Effective()
		lang := contentLang(r.URL.Query().Get("lang"), langs)
		p = newPublicIncident(inc, lang, langs)
	}

	s.writePublic(w, r, p, err)
}

// publicArchive answers a page of all incidents of a site, most recent
// activity first, with the number of pages.
func (s *Server) publicArchive(w http.ResponseWriter, r *http.Request) {
	site, err := s.publicSiteOf(r)
	var incidents []model.Incident
	if err == nil {
		incidents, err = s.db.Incidents(site.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	q := r.URL.Query()
	byActivity(incidents)
	from, to, pages, ok := paginate(q.Get("page"), len(incidents), archivePageSize)
	if !ok {
		e := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	langs := site.Languages.Effective()
	lang := contentLang(q.Get("lang"), langs)
	list := []publicIncident{}
	for _, inc := range incidents[from:to] {
		list = append(list, newPublicIncident(&inc, lang, langs))
	}

	body := map[string]any{
		"incidents": list,
		"pages":     pages,
	}
	s.writePublic(w, r, body, nil)
}

// previewSite answers the payload of a site for the admin console: always
// in full, with the panels' errors.
func (s *Server) previewSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.db.Site(r.PathValue("site"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	p, err := s.sitePayload(site, r.URL.Query().Get("lang"), "", true)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, p)
}

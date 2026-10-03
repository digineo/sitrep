package server

import (
	"net/http"
	"time"
	"uuid"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// payload is a site's data for its page. Responses to requests with a
// cursor only carry the sections that changed after it; status is always
// included.
type payload struct {
	Cursor string                  `json:"cursor"`
	Status string                  `json:"status"`
	Site   *sitePayload            `json:"site,omitempty"`
	Panels map[string]panelPayload `json:"panels,omitempty"`
}

type sitePayload struct {
	Name      string      `json:"name"`
	Theme     string      `json:"theme"`
	Languages []string    `json:"languages"`
	Timezone  string      `json:"timezone"`
	Panels    []panelInfo `json:"panels"`
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
	Error     string     `json:"error,omitempty"` // for admins only
}

// sitePayload builds the payload of site in lang with the sections changed
// after the cursor since, or all of them. Admin payloads carry the panels'
// errors.
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

	langs := site.Languages.Effective()
	lang = contentLang(lang, langs)
	p := payload{
		Cursor: s.poller.Cursor(seq),
		Status: s.status(panels).Overall,
		Panels: map[string]panelPayload{},
	}
	if !incremental || s.poller.SiteSeq(site.ID) > after {
		p.Site = &sitePayload{
			Name:      site.Name.Resolve(lang, langs),
			Theme:     site.Theme,
			Languages: langs.Enabled,
			Timezone:  site.Timezone,
			Panels:    []panelInfo{},
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

// publicSite answers the site payload for visitors, in the requested
// language.
func (s *Server) publicSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("site")
	if !validID(id) {
		e := apierr.New(http.StatusBadRequest, apierr.InvalidValue)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	site, err := s.db.Site(id)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	q := r.URL.Query()
	p, err := s.sitePayload(site, q.Get("lang"), q.Get("since"), false)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=10")
	httpx.WriteJSON(w, http.StatusOK, p)
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

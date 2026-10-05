package server

import (
	"net/http"
	"slices"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/poller"
	"github.com/digineo/sitrep/internal/process"
)

// siteSummary is a site in the admin console's lists.
type siteSummary struct {
	ID           string          `json:"id"`
	Name         model.Text      `json:"name"`
	Languages    model.Languages `json:"languages"`
	Route        model.Route     `json:"route"`
	Availability string          `json:"availability"`
	Status       siteStatus      `json:"status"`
	// Missing counts the site, panels and incidents with missing
	// translations.
	Missing int `json:"missing"`
	// Landing is whether the site is promoted to the base domains.
	Landing bool `json:"landing,omitempty"`
}

// siteStatus is a site's status and its parts.
type siteStatus struct {
	Overall   string `json:"overall"`
	Panels    string `json:"panels"`
	Incidents string `json:"incidents"`
}

// panelState is the state a status panel contributes to its site's
// status: the threshold state while fresh, else unknown.
func panelState(e poller.Entry, ok bool) string {
	if !ok || e.Pending() || e.Stale {
		return model.StateUnknown
	}
	return e.Data.State
}

// status computes a site's status from its status panels and its ongoing
// incidents: a critical one means down, any other degraded. Paused sites
// are not polled, so only their incidents count.
func (s *Server) status(
	site *model.Site,
	panels []model.Panel,
	incidents []model.Incident,
) siteStatus {
	var states []string
	for _, p := range panels {
		if p.Type == model.PanelStatus {
			e, ok := s.poller.Entry(p.ID)
			states = append(states, panelState(e, ok))
		}
	}

	st := siteStatus{
		Panels:    process.Worst(states...),
		Incidents: model.StateOperational,
	}
	for _, inc := range incidents {
		if inc.Phase() != model.PhaseOngoing {
			continue
		}

		state := model.StateDegraded
		if inc.Severity() == model.SeverityCritical {
			state = model.StateDown
		}
		st.Incidents = process.Worst(st.Incidents, state)
	}

	st.Overall = process.Worst(st.Panels, st.Incidents)
	if site.Availability == model.AvailabilityPaused {
		st.Overall = st.Incidents
	}
	return st
}

// missing reports whether any of the texts misses a translation.
func missing(l model.Languages, texts ...model.Text) bool {
	return slices.ContainsFunc(texts, func(t model.Text) bool {
		return t.Missing(l)
	})
}

// incidentMissing reports whether an incident's title or an update's
// description misses a translation.
func incidentMissing(inc model.Incident, l model.Languages) bool {
	texts := []model.Text{inc.Title}
	for _, u := range inc.Updates {
		texts = append(texts, u.Description)
	}
	return missing(l, texts...)
}

// listSites lists the sites the request's account holds a role on.
func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	snap, err := s.db.Snapshot()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	settings, err := s.db.Settings()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	panels := map[string][]model.Panel{}
	for _, p := range snap.Panels {
		panels[p.Site] = append(panels[p.Site], p)
	}

	incidents := map[string][]model.Incident{}
	for _, inc := range snap.Incidents {
		incidents[inc.Site] = append(incidents[inc.Site], inc)
	}

	acc := auth.Account(r.Context())
	sites := []siteSummary{}
	for _, site := range snap.Sites {
		if !acc.Can(site.ID, model.RoleResponder) {
			continue
		}

		langs := site.Languages.Effective()
		sum := siteSummary{
			ID:           site.ID,
			Name:         site.Name,
			Languages:    langs,
			Route:        site.Route,
			Availability: site.Availability,
			Status:       s.status(&site, panels[site.ID], incidents[site.ID]),
			Landing:      site.ID == settings.LandingSite,
		}
		if missing(langs, append(site.Legal.Texts(), site.Name)...) {
			sum.Missing++
		}

		for _, p := range panels[site.ID] {
			if missing(langs, p.Title, p.Description, p.Unit, p.Legend) {
				sum.Missing++
			}
		}

		for _, inc := range incidents[site.ID] {
			if incidentMissing(inc, langs) {
				sum.Missing++
			}
		}

		sites = append(sites, sum)
	}

	httpx.WriteJSON(w, http.StatusOK, sites)
}

// readSite reads and validates a submitted site.
func (s *Server) readSite(
	w http.ResponseWriter,
	r *http.Request,
) (*model.Site, error) {
	var site model.Site
	if err := httpx.ReadJSON(w, r, &site); err != nil {
		return nil, err
	}
	site.Normalize()
	return &site, site.Validate(s.cfg.BaseDomains)
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.readSite(w, r)
	if err == nil {
		err = s.db.CreateSite(site)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(site.ID)
	httpx.WriteJSON(w, http.StatusCreated, site)
}

func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.db.Site(r.PathValue("site"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	site.Languages = site.Languages.Effective()
	httpx.WriteJSON(w, http.StatusOK, site)
}

func (s *Server) putSite(w http.ResponseWriter, r *http.Request) {
	site, err := s.readSite(w, r)
	if err == nil {
		site.ID = r.PathValue("site")
		err = s.db.UpdateSite(site)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(site.ID)
	httpx.WriteJSON(w, http.StatusOK, site)
}

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("site")
	if err := s.db.DeleteSite(id); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	s.poller.SiteChanged(id)
	w.WriteHeader(http.StatusNoContent)
}

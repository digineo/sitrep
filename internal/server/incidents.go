package server

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"time"
	"uuid"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/markdown"
	"github.com/digineo/sitrep/internal/model"
)

// Page sizes of incident lists.
const (
	adminPageSize   = 50
	archivePageSize = 20
)

// byActivity sorts incidents by their last activity, most recent first.
func byActivity(incidents []model.Incident) {
	slices.SortStableFunc(incidents, func(a, b model.Incident) int {
		return cmp.Or(
			b.LastActivity().Compare(a.LastActivity()),
			cmp.Compare(b.ID, a.ID),
		)
	})
}

// paginate returns the bounds of a page of n items and the number of
// pages, at least one. raw is the requested page number, empty for the
// first; ok is false for anything else than a page that exists.
func paginate(raw string, n, size int) (from, to, pages int, ok bool) {
	pages = max(1, (n+size-1)/size)
	page := 1
	if raw != "" {
		var err error
		if page, err = strconv.Atoi(raw); err != nil || page < 1 || page > pages {
			return 0, 0, pages, false
		}
	}

	from = (page - 1) * size
	return from, min(from+size, n), pages, true
}

// incidentView is an incident as the admin console sees it, with derived
// values and rendered descriptions.
type incidentView struct {
	model.Incident
	Updates      []updateView `json:"updates"` // replaces the embedded field
	Status       string       `json:"status"`
	Severity     string       `json:"severity,omitempty"`
	Phase        string       `json:"phase"`
	LastActivity time.Time    `json:"lastActivity"`
}

type updateView struct {
	model.Update
	// the description rendered, per language
	HTML model.Text `json:"html,omitempty"`
}

func newIncidentView(inc *model.Incident) incidentView {
	v := incidentView{
		Incident:     *inc,
		Status:       inc.Status(),
		Severity:     inc.Severity(),
		Phase:        inc.Phase(),
		LastActivity: inc.LastActivity(),
	}
	for _, u := range inc.Updates {
		uv := updateView{Update: u}
		for lang, src := range u.Description {
			if uv.HTML == nil {
				uv.HTML = model.Text{}
			}
			uv.HTML[lang] = markdown.HTML(src)
		}

		v.Updates = append(v.Updates, uv)
	}
	return v
}

// person returns the admin who sent the request.
func person(r *http.Request) model.Person {
	acc := auth.Account(r.Context())
	return model.Person{
		Subject:     acc.Subject,
		DisplayName: acc.DisplayName,
	}
}

// updateInput is a submitted update. A missing time means now for new
// updates and unchanged for edited ones.
type updateInput struct {
	At          *time.Time `json:"at"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	Description model.Text `json:"description"`
}

// set writes the submitted fields to u.
func (in updateInput) set(u *model.Update) {
	u.Status = in.Status
	u.Severity = in.Severity
	u.Description = in.Description.Normalize()
	if in.At != nil {
		u.At = in.At.UTC()
	}
}

// validate checks the submitted update on its own; errors are added at
// prefix+field.
func (in updateInput) validate(f *apierr.Fields, prefix string, l model.Languages) {
	var u model.Update
	in.set(&u)
	u.Validate(f, prefix, l)
}

// now returns the time of an update that comes without one.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	incidents, err := s.db.Incidents(r.PathValue("site"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	byActivity(incidents)
	q := r.URL.Query()
	from, to, pages, ok := paginate(q.Get("page"), len(incidents), adminPageSize)
	if !ok {
		e := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	views := []incidentView{}
	for _, inc := range incidents[from:to] {
		views = append(views, newIncidentView(&inc))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"incidents": views,
		"pages":     pages,
	})
}

func (s *Server) createIncident(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title  model.Text  `json:"title"`
		Update updateInput `json:"update"`
	}
	site, err := s.readForSite(w, r, &in)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := site.Languages.Effective()
	var f apierr.Fields
	model.ValidateTitle(&f, in.Title.Normalize(), langs)
	in.Update.validate(&f, "update.", langs)

	by := person(r)
	u := model.Update{
		ID:        uuid.NewV7().String(),
		At:        now(),
		Author:    by,
		CreatedAt: time.Now().UTC(),
	}
	in.Update.set(&u)
	inc := &model.Incident{
		Site:    site.ID,
		Title:   in.Title.Normalize(),
		Updates: []model.Update{u},
		Author:  by,
	}

	err = f.Err()
	if err == nil {
		err = s.db.CreateIncident(inc)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.IncidentsChanged(site.ID)
	httpx.WriteJSON(w, http.StatusCreated, newIncidentView(inc))
}

// readForSite reads the submitted body of a request for a site into in and
// returns the site.
func (s *Server) readForSite(
	w http.ResponseWriter,
	r *http.Request,
	in any,
) (*model.Site, error) {
	site, err := s.db.Site(r.PathValue("site"))
	if err != nil {
		return nil, err
	}
	return site, httpx.ReadJSON(w, r, in)
}

func (s *Server) getIncident(w http.ResponseWriter, r *http.Request) {
	inc, err := s.db.Incident(r.PathValue("site"), r.PathValue("incident"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, newIncidentView(inc))
}

func (s *Server) deleteIncident(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	if err := s.db.DeleteIncident(site, r.PathValue("incident")); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	s.poller.IncidentsChanged(site)
	w.WriteHeader(http.StatusNoContent)
}

// change applies change to the request's incident, unless err is set,
// and answers with the changed incident. An incident left without updates
// is gone: the answer is 204.
func (s *Server) change(
	w http.ResponseWriter,
	r *http.Request,
	err error,
	change func(*model.Incident) error,
) {
	site := r.PathValue("site")
	var inc *model.Incident
	if err == nil {
		inc, err = s.db.ChangeIncident(site, r.PathValue("incident"), change)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.IncidentsChanged(site)
	if inc == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newIncidentView(inc))
}

// putIncident replaces the title.
func (s *Server) putIncident(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title model.Text `json:"title"`
	}
	site, err := s.readForSite(w, r, &in)
	title := in.Title.Normalize()
	if err == nil {
		var f apierr.Fields
		model.ValidateTitle(&f, title, site.Languages.Effective())
		err = f.Err()
	}

	s.change(w, r, err, func(inc *model.Incident) error {
		inc.Title = title
		return nil
	})
}

// readUpdate reads and validates a submitted update.
func (s *Server) readUpdate(
	w http.ResponseWriter,
	r *http.Request,
) (updateInput, error) {
	var in updateInput
	site, err := s.readForSite(w, r, &in)
	if err != nil {
		return in, err
	}

	var f apierr.Fields
	in.validate(&f, "", site.Languages.Effective())
	return in, f.Err()
}

func (s *Server) addUpdate(w http.ResponseWriter, r *http.Request) {
	in, err := s.readUpdate(w, r)
	u := model.Update{
		ID:        uuid.NewV7().String(),
		At:        now(),
		Author:    person(r),
		CreatedAt: time.Now().UTC(),
	}
	in.set(&u)
	s.change(w, r, err, func(inc *model.Incident) error {
		inc.Updates = append(inc.Updates, u)
		return nil
	})
}

// updateIndex returns the index of the incident's update with the ID.
func updateIndex(inc *model.Incident, id string) (int, error) {
	i := slices.IndexFunc(inc.Updates, func(u model.Update) bool {
		return u.ID == id
	})
	if i < 0 {
		return 0, apierr.New(http.StatusNotFound, apierr.NotFound)
	}
	return i, nil
}

func (s *Server) putUpdate(w http.ResponseWriter, r *http.Request) {
	in, err := s.readUpdate(w, r)
	by, at := person(r), time.Now().UTC()
	s.change(w, r, err, func(inc *model.Incident) error {
		i, err := updateIndex(inc, r.PathValue("update"))
		if err != nil {
			return err
		}

		in.set(&inc.Updates[i])
		inc.Updates[i].EditedBy, inc.Updates[i].EditedAt = &by, &at
		return nil
	})
}

// deleteUpdate deletes an update; deleting the last one deletes the
// incident.
func (s *Server) deleteUpdate(w http.ResponseWriter, r *http.Request) {
	s.change(w, r, nil, func(inc *model.Incident) error {
		i, err := updateIndex(inc, r.PathValue("update"))
		if err != nil {
			return err
		}
		inc.Updates = slices.Delete(inc.Updates, i, i+1)
		return nil
	})
}

// previewMarkdown renders Markdown for the console's editors.
func (s *Server) previewMarkdown(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	html := markdown.HTML(in.Text)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"html": html})
}

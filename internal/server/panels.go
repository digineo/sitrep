package server

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/poller"
	"github.com/digineo/sitrep/internal/process"
)

func (s *Server) listPanels(w http.ResponseWriter, r *http.Request) {
	panels, err := s.db.Panels(r.PathValue("site"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, append([]model.Panel{}, panels...))
}

func (s *Server) getPanel(w http.ResponseWriter, r *http.Request) {
	p, err := s.db.Panel(r.PathValue("site"), r.PathValue("panel"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// checkPanel normalizes and validates a submitted panel of site, including
// whether its data source supports the panel type, and returns the data
// source.
func (s *Server) checkPanel(
	p *model.Panel,
	site *model.Site,
	requireTitle bool,
) (*model.DataSource, error) {
	p.Site = site.ID
	p.Normalize()
	langs := site.Languages.Effective()
	var f apierr.Fields
	if e, ok := errors.AsType[*apierr.Error](p.Validate(langs, requireTitle)); ok {
		f = e.Fields
	}

	var ds *model.DataSource
	if p.DataSource != "" {
		var err error
		ds, err = s.db.DataSource(p.DataSource)
		e, ok := errors.AsType[*apierr.Error](err)
		if ok && e.Status == http.StatusNotFound {
			f.Add("datasource", apierr.NotFound)
		} else if err != nil {
			return nil, err
		} else if t := datasource.Get(ds.Type); t == nil ||
			!datasource.Supports(t, p.Type) {
			f.Add("datasource", apierr.UnsupportedType)
		}
	}
	return ds, f.Err()
}

// readPanel reads and validates a panel submitted for the request's site.
func (s *Server) readPanel(
	w http.ResponseWriter,
	r *http.Request,
) (*model.Panel, error) {
	site, err := s.db.Site(r.PathValue("site"))
	if err != nil {
		return nil, err
	}

	var p model.Panel
	if err := httpx.ReadJSON(w, r, &p); err != nil {
		return nil, err
	}

	_, err = s.checkPanel(&p, site, true)
	return &p, err
}

func (s *Server) createPanel(w http.ResponseWriter, r *http.Request) {
	p, err := s.readPanel(w, r)
	if err == nil {
		err = s.db.CreatePanel(p)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(p.Site)
	httpx.WriteJSON(w, http.StatusCreated, p)
}

func (s *Server) putPanel(w http.ResponseWriter, r *http.Request) {
	p, err := s.readPanel(w, r)
	if err == nil {
		p.ID = r.PathValue("panel")
		err = s.db.UpdatePanel(p)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(p.Site)
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (s *Server) deletePanel(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	if err := s.db.DeletePanel(site, r.PathValue("panel")); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	s.poller.SiteChanged(site)
	w.WriteHeader(http.StatusNoContent)
}

// reorderPanels sets the display order. The request lists every panel of
// the site once.
func (s *Server) reorderPanels(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Panels []string `json:"panels"`
	}
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	site := r.PathValue("site")
	if err := s.db.ReorderPanels(site, in.Panels); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(site)
	w.WriteHeader(http.StatusNoContent)
}

// warning is a hint about a preview's result, e.g. dropped series.
type warning struct {
	Code  string `json:"code"`
	Count int    `json:"count,omitempty"`
}

type previewResult struct {
	Data     any       `json:"data,omitempty"`
	Warnings []warning `json:"warnings"`
	Error    string    `json:"error,omitempty"`
}

// previewPanel evaluates an unsaved panel like a poll, without touching the
// cache, and returns its data in the requested language.
func (s *Server) previewPanel(w http.ResponseWriter, r *http.Request) {
	site, err := s.db.Site(r.PathValue("site"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	var in struct {
		Panel model.Panel `json:"panel"`
		Lang  string      `json:"lang"`
	}
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	ds, err := s.checkPanel(&in.Panel, site, false)
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	langs := site.Languages.Effective()
	res := previewResult{Warnings: []warning{}}
	ctx := r.Context()
	d, err := poller.Evaluate(ctx, s.cfg.SecretKey, in.Panel, *ds, time.Now())
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Data = panelData(in.Panel, d, contentLang(in.Lang, langs), langs, true)
		if d.Dropped > 0 {
			res.Warnings = append(res.Warnings, warning{
				Code:  "series_dropped",
				Count: d.Dropped,
			})
		}
	}

	httpx.WriteJSON(w, http.StatusOK, res)
}

// contentLang returns lang if the site has it enabled, else its primary
// language.
func contentLang(lang string, l model.Languages) string {
	if slices.Contains(l.Enabled, lang) {
		return lang
	}
	return l.Primary
}

// panelData returns the public data of a panel: values with raw numbers,
// series with their names in lang, and no labels.
func panelData(
	p model.Panel,
	d process.Data,
	lang string,
	l model.Languages,
	fresh bool,
) any {
	switch p.Type {
	case model.PanelStatus:
		state := d.State
		if !fresh {
			state = model.StateUnknown
		}
		return map[string]any{"state": state}
	case model.PanelTimeseries:
		times := make([]int64, len(d.Times))
		for i, t := range d.Times {
			times[i] = t.Unix()
		}

		type series struct {
			Name   string     `json:"name"`
			Values []*float64 `json:"values"`
		}

		names := d.Names(p, lang, l)
		out := make([]series, len(d.Series))
		for i, s := range d.Series {
			out[i] = series{
				Name:   names[i],
				Values: s.Values,
			}
		}
		return map[string]any{
			"times":  times,
			"series": out,
		}
	}
	return map[string]any{"value": d.Value}
}

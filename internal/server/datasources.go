package server

import (
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"uuid"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// typeView describes a data source type to the admin console.
type typeView struct {
	ID         string             `json:"id"`
	Fields     []datasource.Field `json:"fields"`
	PanelTypes []string           `json:"panelTypes"`
	Editor     string             `json:"editor,omitempty"`
}

func (s *Server) listTypes(w http.ResponseWriter, _ *http.Request) {
	views := []typeView{}
	for _, id := range datasource.IDs() {
		t := datasource.Get(id)
		v := typeView{
			ID:         id,
			Fields:     t.Fields(),
			PanelTypes: t.PanelTypes(),
		}
		if e, ok := t.(datasource.Editor); ok {
			v.Editor = e.Editor()
		}

		views = append(views, v)
	}

	httpx.WriteJSON(w, http.StatusOK, views)
}

// dataSourceView is a data source as the admin console sees it: secrets
// are only reported as set.
type dataSourceView struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config"`
	Secrets []string          `json:"secrets"`
	Summary string            `json:"summary"`
	Usable  bool              `json:"usable"`
	Panels  int               `json:"panels"`
}

func (s *Server) dataSourceView(ds model.DataSource, panels int) dataSourceView {
	v := dataSourceView{
		ID:      ds.ID,
		Name:    ds.Name,
		Type:    ds.Type,
		Config:  ds.Config,
		Panels:  panels,
		Secrets: append([]string{}, slices.Sorted(maps.Keys(ds.Secrets))...),
	}

	cfg, err := datasource.Open(ds, s.cfg.SecretKey)
	if t := datasource.Get(ds.Type); t != nil && err == nil {
		v.Summary = t.Summary(cfg)
		v.Usable = true
	}
	return v
}

func (s *Server) listDataSources(w http.ResponseWriter, r *http.Request) {
	snap, err := s.db.Snapshot()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	panels := map[string]int{}
	for _, p := range snap.Panels {
		panels[p.DataSource]++
	}

	views := []dataSourceView{}
	for _, ds := range snap.DataSources {
		views = append(views, s.dataSourceView(ds, panels[ds.ID]))
	}

	httpx.WriteJSON(w, http.StatusOK, views)
}

func (s *Server) getDataSource(w http.ResponseWriter, r *http.Request) {
	ds, err := s.db.DataSource(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.dataSourceView(*ds, 0))
}

// dataSourceInput is a submitted data source. Secret fields that are
// omitted from Config keep their value; empty ones are cleared.
type dataSourceInput struct {
	ID     string            `json:"id,omitempty"` // only for connection tests
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// apply validates in and writes it to ds. A set type must match ds.Type.
func (s *Server) apply(
	ds *model.DataSource,
	in dataSourceInput,
	needName bool,
) error {
	var f apierr.Fields
	if in.Type != ds.Type || datasource.Get(ds.Type) == nil {
		f.Add("type", apierr.InvalidValue)
		return f.Err()
	}

	if needName {
		ds.Name = datasource.ValidateName(&f, in.Name)
	}

	if err := datasource.Apply(&f, ds, in.Config, s.cfg.SecretKey); err != nil {
		return err
	}
	return f.Err()
}

func (s *Server) createDataSource(w http.ResponseWriter, r *http.Request) {
	var in dataSourceInput
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	ds := &model.DataSource{
		ID:   uuid.NewV7().String(),
		Type: in.Type,
	}
	if err := s.apply(ds, in, true); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	if err := s.db.CreateDataSource(ds); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, s.dataSourceView(*ds, 0))
}

func (s *Server) putDataSource(w http.ResponseWriter, r *http.Request) {
	var in dataSourceInput
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	ds, err := s.db.DataSource(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	if in.Type == "" {
		in.Type = ds.Type
	}

	if err := s.apply(ds, in, true); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	if err := s.db.UpdateDataSource(ds); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.Reconcile()
	httpx.WriteJSON(w, http.StatusOK, s.dataSourceView(*ds, 0))
}

func (s *Server) deleteDataSource(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteDataSource(r.PathValue("id")); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testResult is the outcome of a connection test: a short detail on
// success, else the data source's error text.
type testResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) test(w http.ResponseWriter, r *http.Request, ds model.DataSource) {
	res := testResult{OK: true}
	cfg, err := datasource.Open(ds, s.cfg.SecretKey)
	if err == nil {
		res.Detail, err = datasource.Get(ds.Type).Test(r.Context(), cfg)
	}
	if err != nil {
		res = testResult{Error: err.Error()}
	}

	httpx.WriteJSON(w, http.StatusOK, res)
}

// testDataSource tests the stored configuration.
func (s *Server) testDataSource(w http.ResponseWriter, r *http.Request) {
	ds, err := s.db.DataSource(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}
	s.test(w, r, *ds)
}

// testUnsaved tests a submitted configuration. With an ID, secrets that
// are not submitted come from the stored data source.
func (s *Server) testUnsaved(w http.ResponseWriter, r *http.Request) {
	var in dataSourceInput
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	ds := &model.DataSource{
		ID:   uuid.NewV7().String(),
		Type: in.Type,
	}
	if in.ID != "" {
		var err error
		if ds, err = s.db.DataSource(in.ID); err != nil {
			httpx.WriteError(w, r, s.log, err)
			return
		}
	}

	if err := s.apply(ds, in, false); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.test(w, r, *ds)
}

// dataSourceRoute serves the admin routes of a data source's type.
func (s *Server) dataSourceRoute(w http.ResponseWriter, r *http.Request) {
	ds, err := s.db.DataSource(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	router, ok := datasource.Get(ds.Type).(datasource.Router)
	if !ok || r.PathValue("type") != ds.Type {
		notFound := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, notFound)
		return
	}

	cfg, err := datasource.Open(*ds, s.cfg.SecretKey)
	if err != nil {
		unusable := apierr.New(http.StatusConflict, apierr.DataSourceUnusable)
		httpx.WriteError(w, r, s.log, unusable)
		return
	}

	prefix := "/api/admin/datasources/" + ds.ID + "/" + ds.Type
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	if err := router.ServeAdmin(w, r, cfg, rest); err != nil {
		e, ok := errors.AsType[*apierr.Error](err)
		if ok && e.Status == http.StatusBadGateway {
			s.log.Warn("a data source request failed",
				slog.String("datasource", ds.ID),
				xlog.Error(err))
		}

		httpx.WriteError(w, r, s.log, err)
	}
}

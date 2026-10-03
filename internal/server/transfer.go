package server

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// documentVersion is the version of the export format.
const documentVersion = 1

// document is a site with its panels in display order, as exported to
// YAML. Panels name their data source, since IDs differ between instances.
type document struct {
	Version    int `yaml:"version"`
	model.Site `yaml:",inline"`
	Panels     []documentPanel `yaml:"panels"`
}

type documentPanel struct {
	model.Panel `yaml:",inline"`
	DataSource  string `yaml:"datasource"`
}

// exportSite answers the YAML document of a site, as a download named
// after its route.
func (s *Server) exportSite(w http.ResponseWriter, r *http.Request) {
	snap, err := s.db.Snapshot()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	doc := document{
		Version: documentVersion,
		Panels:  []documentPanel{},
	}
	for _, site := range snap.Sites {
		if site.ID == r.PathValue("site") {
			doc.Site = site
		}
	}
	if doc.ID == "" {
		e := apierr.New(http.StatusNotFound, apierr.NotFound)
		httpx.WriteError(w, r, s.log, e)
		return
	}

	names := map[string]string{}
	for _, ds := range snap.DataSources {
		names[ds.ID] = ds.Name
	}

	for _, p := range snap.Panels {
		if p.Site == doc.ID {
			doc.Panels = append(doc.Panels, documentPanel{
				Panel:      p,
				DataSource: names[p.DataSource],
			})
		}
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	base := cmp.Or(doc.Route.Slug, doc.Route.Domain)
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.yaml"`)
	_, _ = w.Write(out.Bytes())
}

// importSite creates a site from a YAML document or, for a request with a
// site, replaces that site's settings and panels.
func (s *Server) importSite(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		err = apierr.New(http.StatusRequestEntityTooLarge, apierr.TooLarge)
	}

	var site *model.Site
	var panels []model.Panel
	if err == nil {
		site, panels, err = s.readDocument(body)
	}

	status := http.StatusCreated
	if id := r.PathValue("site"); err == nil && id != "" {
		site.ID = id
		status = http.StatusOK
	}

	if err == nil {
		err = s.db.ImportSite(site, panels)
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.poller.SiteChanged(site.ID)
	httpx.WriteJSON(w, status, site)
}

// readDocument parses and validates a YAML document. Only its first YAML
// document is read, and unknown keys are rejected. The panels refer to
// their data sources by ID.
func (s *Server) readDocument(
	body []byte,
) (*model.Site, []model.Panel, error) {
	var head struct {
		Version any `yaml:"version"`
	}
	if err := yaml.Unmarshal(body, &head); err != nil {
		return nil, nil, yamlError(err)
	}

	switch head.Version {
	case documentVersion:
	case nil:
		return nil, nil, apierr.Fields{{
			Path: "version",
			Code: apierr.Required,
		}}.Err()
	default:
		return nil, nil, apierr.Fields{{
			Path: "version",
			Code: apierr.UnsupportedVersion,
		}}.Err()
	}

	var doc document
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, yamlError(err)
	}

	sources, err := s.db.DataSources()
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]model.DataSource{}
	for _, ds := range sources {
		byName[strings.ToLower(ds.Name)] = ds
	}

	var f apierr.Fields
	site := doc.Site
	site.Normalize()
	addFields(&f, "", site.Validate(s.cfg.BaseDomains))

	panels := []model.Panel{}
	for i, dp := range doc.Panels {
		prefix := fmt.Sprintf("panels[%d].", i)
		p := dp.Panel
		ds, found := byName[strings.ToLower(dp.DataSource)]
		p.DataSource = cmp.Or(ds.ID, dp.DataSource)
		p.Normalize()
		addFields(&f, prefix, p.Validate(site.Languages.Effective(), true))
		switch t := datasource.Get(ds.Type); {
		case dp.DataSource != "" && !found:
			f.Add(prefix+"datasource", apierr.NotFound)
		case found && (t == nil || !datasource.Supports(t, p.Type)):
			f.Add(prefix+"datasource", apierr.UnsupportedType)
		}

		panels = append(panels, p)
	}
	return &site, panels, f.Err()
}

var yamlLine = regexp.MustCompile(`line (\d+)`)

// yamlError reports a document that cannot be parsed, with the line of the
// first problem if it is known.
func yamlError(err error) error {
	e := apierr.New(http.StatusBadRequest, apierr.InvalidYAML)
	if m := yamlLine.FindStringSubmatch(err.Error()); m != nil {
		line, _ := strconv.Atoi(m[1])
		e.Details = map[string]int{"line": line}
	}
	return e
}

// addFields adds the fields of a validation error to f, below prefix.
func addFields(f *apierr.Fields, prefix string, err error) {
	if e, ok := errors.AsType[*apierr.Error](err); ok {
		for _, field := range e.Fields {
			f.Add(prefix+field.Path, field.Code)
		}
	}
}

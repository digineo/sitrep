// Package datasource defines the contract of data source types and the
// configuration handling they share. A type lives in its own package,
// registers itself with Register in an init function, and is linked into
// the binary by a blank import in cmd/sitrep.
package datasource

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/digineo/sitrep/internal/model"
)

// MaxResponse is the size limit of responses read from a data source.
const MaxResponse = 1 << 20

// Type is a kind of data source, e.g. Prometheus.
type Type interface {
	// Fields describes the configuration. The admin console renders its
	// form from it.
	Fields() []Field
	// PanelTypes lists the panel types the type can feed.
	PanelTypes() []string
	// Evaluate runs the panel's query at now.
	Evaluate(
		ctx context.Context,
		cfg Config,
		p model.Panel,
		now time.Time,
	) (Result, error)
	// Test checks the connection and returns a short technical detail.
	Test(ctx context.Context, cfg Config) (string, error)
	// Summary describes the configuration in admin lists, without secrets.
	Summary(cfg Config) string
}

// Router is implemented by types with admin-only HTTP routes below
// /api/admin/datasources/{id}/{type}/. path is the rest of the URL path,
// starting with a slash. If ServeAdmin returns an error, it has not written
// a response; an *apierr.Error in the error's chain is answered as such.
type Router interface {
	ServeAdmin(
		w http.ResponseWriter,
		r *http.Request,
		cfg Config,
		path string,
	) error
}

// Editor is implemented by types that have their own query editor in the
// admin console.
type Editor interface {
	Editor() string
}

// Config is a data source's configuration by field name, with secrets in
// plain text.
type Config map[string]string

// Result is the normalized outcome of an evaluation: a scalar, labeled
// samples of an instant, or labeled series over a shared time axis.
type Result struct {
	Scalar  *float64
	Samples []Sample
	Times   []time.Time
	Series  []Series
}

// Sample is a labeled value.
type Sample struct {
	Labels map[string]string
	Value  float64
}

// Series is a labeled list of values, one per time of the result. Missing
// values are NaN.
type Series struct {
	Labels map[string]string
	Values []float64
}

var types = map[string]Type{}

// Register makes a type available under id.
func Register(id string, t Type) {
	types[id] = t
}

// Get returns the type registered as id, or nil.
func Get(id string) Type {
	return types[id]
}

// IDs returns the registered type IDs, sorted.
func IDs() []string {
	ids := make([]string, 0, len(types))
	for id := range types {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// Supports reports whether t can feed panels of panelType.
func Supports(t Type, panelType string) bool {
	return slices.Contains(t.PanelTypes(), panelType)
}

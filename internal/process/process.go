// Package process turns the normalized results of data sources into panel
// data: reduce and thresholds for stat and status panels, series handling
// for timeseries panels. It works the same for every data source type.
package process

import (
	"cmp"
	"errors"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/i18n"
	"github.com/digineo/sitrep/internal/model"
)

// MaxSeries is the number of series a timeseries panel keeps.
const MaxSeries = 20

// Data is a panel's processed result.
type Data struct {
	// Value is the reduced value of stat and status panels, or nil for "no
	// data".
	Value *float64
	// State is the threshold state of status panels.
	State string
	// Times and Series are the data of timeseries panels.
	Times  []time.Time
	Series []Series
	// Dropped counts the series beyond MaxSeries.
	Dropped int
}

// Series is one line of a chart. Its labels never reach public clients;
// they only feed the legend template.
type Series struct {
	Labels map[string]string
	Values []*float64 // nil is a gap
}

var (
	errScalarOrSamples = errors.New(
		"the query must return a scalar or an instant vector",
	)
	errSeries = errors.New("the query must return series over time")
)

// Process applies the panel's options to a result.
func Process(p model.Panel, r datasource.Result) (Data, error) {
	if p.Type == model.PanelTimeseries {
		if r.Times == nil {
			return Data{}, errSeries
		}
		return series(r), nil
	}

	if r.Times != nil {
		return Data{}, errScalarOrSamples
	}

	d := Data{Value: reduce(p.Reduce, r)}
	if p.Type == model.PanelStatus {
		d.State = state(p.Thresholds, d.Value)
	}
	return d, nil
}

// reduce returns the scalar, or reduces the samples sorted by labels. No
// samples and non-finite results are "no data".
func reduce(how string, r datasource.Result) *float64 {
	if r.Scalar != nil {
		return finite(*r.Scalar)
	}

	if len(r.Samples) == 0 {
		return nil
	}

	byLabels := func(a, b datasource.Sample) int {
		return compareLabels(a.Labels, b.Labels)
	}

	samples := slices.SortedFunc(slices.Values(r.Samples), byLabels)
	v := samples[0].Value
	switch how {
	case "last":
		v = samples[len(samples)-1].Value
	case "sum", "avg":
		v = 0
		for _, s := range samples {
			v += s.Value
		}

		if how == "avg" {
			v /= float64(len(samples))
		}
	case "min":
		for _, s := range samples[1:] {
			v = math.Min(v, s.Value)
		}
	case "max":
		for _, s := range samples[1:] {
			v = math.Max(v, s.Value)
		}
	}
	return finite(v)
}

func finite(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

// state returns the state of the first matching threshold, operational if
// none matches, or unknown without a value.
func state(thresholds []model.Threshold, v *float64) string {
	if v == nil {
		return model.StateUnknown
	}
	for _, t := range thresholds {
		if t.Matches(*v) {
			return t.State
		}
	}
	return model.StateOperational
}

// series sorts the series by labels, keeps the first MaxSeries and turns
// non-finite values into gaps.
func series(r datasource.Result) Data {
	byLabels := func(a, b datasource.Series) int {
		return compareLabels(a.Labels, b.Labels)
	}

	all := slices.SortedFunc(slices.Values(r.Series), byLabels)
	d := Data{
		Times:   r.Times,
		Dropped: max(0, len(all)-MaxSeries),
	}
	for _, s := range all[:min(len(all), MaxSeries)] {
		values := make([]*float64, len(s.Values))
		for i, v := range s.Values {
			values[i] = finite(v)
		}

		d.Series = append(d.Series, Series{
			Labels: s.Labels,
			Values: values,
		})
	}
	return d
}

// compareLabels orders label sets deterministically: by their sorted
// name-value pairs.
func compareLabels(a, b map[string]string) int {
	ka, kb := slices.Sorted(maps.Keys(a)), slices.Sorted(maps.Keys(b))
	for i := range min(len(ka), len(kb)) {
		c := cmp.Or(
			strings.Compare(ka[i], kb[i]),
			strings.Compare(a[ka[i]], b[kb[i]]),
		)
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(ka), len(kb))
}

var legendPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// Names returns the series names in lang: the panel's legend template with
// {{ label }} replaced by label values, or else the panel title, numbered
// when there are several series.
func (d Data) Names(p model.Panel, lang string, l model.Languages) []string {
	tmpl := p.Legend.Resolve(lang, l)
	title := p.Title.Resolve(lang, l)
	names := make([]string, len(d.Series))
	for i, s := range d.Series {
		label := func(m string) string {
			return s.Labels[legendPattern.FindStringSubmatch(m)[1]]
		}

		name := strings.TrimSpace(legendPattern.ReplaceAllStringFunc(tmpl, label))
		switch {
		case name != "":
		case len(d.Series) == 1:
			name = title
		default:
			name = i18n.Get(lang).T("panel.seriesName", map[string]string{
				"title": title,
				"n":     strconv.Itoa(i + 1),
			})
		}

		names[i] = name
	}
	return names
}

var severity = []string{
	model.StateOperational,
	model.StateUnknown,
	model.StateDegraded,
	model.StateDown,
}

// Worst returns the most severe of the states, or operational without any.
func Worst(states ...string) string {
	worst := model.StateOperational
	for _, s := range states {
		if slices.Index(severity, s) > slices.Index(severity, worst) {
			worst = s
		}
	}
	return worst
}

package model

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/digineo/sitrep/internal/apierr"
)

// Panel types.
const (
	PanelStat       = "stat"
	PanelStatus     = "status"
	PanelTimeseries = "timeseries"
)

// Panel states, in ascending severity. Status panels map a value to
// operational, degraded or down; unknown means no usable value.
const (
	StateOperational = "operational"
	StateUnknown     = "unknown"
	StateDegraded    = "degraded"
	StateDown        = "down"
)

var (
	panelTypes      = []string{PanelStat, PanelStatus, PanelTimeseries}
	reducers        = []string{"first", "last", "sum", "avg", "min", "max"}
	thresholdOps    = []string{"<", "<=", ">", ">=", "==", "!="}
	thresholdStates = []string{StateOperational, StateDegraded, StateDown}
	styles          = []string{"line", "area"}
)

// Limits of panel options.
const (
	maxThresholds = 20
	maxDecimals   = 6
	maxPoints     = 11000 // range / step
)

// Panel is one query against a data source plus its presentation. A stored
// panel only carries the options of its type.
type Panel struct {
	ID          string `json:"id"`
	Site        string `json:"site"`
	Type        string `json:"type"`
	Title       Text   `json:"title"`
	Description Text   `json:"description,omitempty"`
	DataSource  string `json:"datasource"`
	Query       string `json:"query"`
	Order       int    `json:"order"`
	// duration; empty means the instance default
	Refresh  string `json:"refresh,omitempty"`
	Revision int64  `json:"revision"`

	// stat and status
	Reduce string `json:"reduce,omitempty"`
	// stat and timeseries
	Decimals *int `json:"decimals,omitempty"`
	Unit     Text `json:"unit,omitempty"`
	// status
	Thresholds []Threshold `json:"thresholds,omitempty"`
	// timeseries
	Range   string `json:"range,omitempty"`
	Step    string `json:"step,omitempty"`
	Style   string `json:"style,omitempty"`
	MinZero bool   `json:"minZero,omitempty"`
	Legend  Text   `json:"legend,omitempty"`
}

// Threshold maps values matching "value <op> Value" to State.
type Threshold struct {
	Op    string  `json:"op"`
	Value float64 `json:"value"`
	State string  `json:"state"`
}

// Matches reports whether v matches the threshold.
func (t Threshold) Matches(v float64) bool {
	switch t.Op {
	case "<":
		return v < t.Value
	case "<=":
		return v <= t.Value
	case ">":
		return v > t.Value
	case ">=":
		return v >= t.Value
	case "==":
		return v == t.Value
	case "!=":
		return v != t.Value
	}
	return false
}

// Normalize trims texts, fills defaults, writes durations in canonical form
// and drops the options of other panel types.
func (p *Panel) Normalize() {
	o := *p
	*p = Panel{
		ID:          o.ID,
		Site:        o.Site,
		Type:        o.Type,
		Title:       o.Title.Normalize(),
		Description: o.Description.Normalize(),
		DataSource:  o.DataSource,
		Query:       strings.TrimSpace(o.Query),
		Order:       o.Order,
		Refresh:     canonicalDuration(o.Refresh),
		Revision:    o.Revision,
	}
	switch p.Type {
	case PanelStat:
		p.Reduce = cmp.Or(o.Reduce, "first")
		p.Decimals = orDefault(o.Decimals, 0)
		p.Unit = o.Unit.Normalize()
	case PanelStatus:
		p.Reduce = cmp.Or(o.Reduce, "first")
		p.Thresholds = o.Thresholds
	case PanelTimeseries:
		p.Range = canonicalDuration(o.Range)
		p.Step = canonicalDuration(o.Step)
		p.Style = cmp.Or(o.Style, "line")
		p.MinZero = o.MinZero
		p.Decimals = orDefault(o.Decimals, 2)
		p.Unit = o.Unit.Normalize()
		p.Legend = o.Legend.Normalize()
	}
}

func orDefault(v *int, def int) *int {
	if v == nil {
		return &def
	}
	return v
}

// canonicalDuration rewrites a valid duration in canonical form and leaves
// anything else for validation to reject.
func canonicalDuration(s string) string {
	if d, err := ParseDuration(s); err == nil {
		return FormatDuration(d)
	}
	return s
}

// Validate checks a normalized panel against the languages of its site.
// Previews of unsaved panels need no title. Whether the data source exists
// and supports the panel type is checked by the caller.
func (p *Panel) Validate(l Languages, requireTitle bool) error {
	var f apierr.Fields
	if !slices.Contains(panelTypes, p.Type) {
		f.Add("type", apierr.InvalidValue)
	}

	validateText(&f, "title", p.Title, l, requireTitle, maxName)
	validateText(&f, "description", p.Description, l, false, maxDescription)

	if p.DataSource == "" {
		f.Add("datasource", apierr.Required)
	}

	switch {
	case p.Query == "":
		f.Add("query", apierr.Required)
	case utf8.RuneCountInString(p.Query) > maxQuery:
		f.Add("query", apierr.TooLong)
	}

	if p.Refresh != "" {
		validateDuration(&f, "refresh", p.Refresh, 5*time.Second, 24*time.Hour)
	}

	if p.Reduce != "" && !slices.Contains(reducers, p.Reduce) {
		f.Add("reduce", apierr.InvalidValue)
	}

	if p.Decimals != nil && (*p.Decimals < 0 || *p.Decimals > maxDecimals) {
		f.Add("decimals", apierr.OutOfRange)
	}

	validateText(&f, "unit", p.Unit, l, false, maxUnit)

	if len(p.Thresholds) > maxThresholds {
		f.Add("thresholds", apierr.TooLong)
	}

	for i, t := range p.Thresholds {
		if !slices.Contains(thresholdOps, t.Op) {
			f.Add(fmt.Sprintf("thresholds[%d].op", i), apierr.InvalidValue)
		}

		if !slices.Contains(thresholdStates, t.State) {
			f.Add(fmt.Sprintf("thresholds[%d].state", i), apierr.InvalidValue)
		}
	}

	if p.Type == PanelTimeseries {
		p.validateRange(&f)
	}

	if p.Style != "" && !slices.Contains(styles, p.Style) {
		f.Add("style", apierr.InvalidValue)
	}

	validateText(&f, "legend", p.Legend, l, false, maxLegend)
	return f.Err()
}

func (p *Panel) validateRange(f *apierr.Fields) {
	if p.Range == "" {
		f.Add("range", apierr.Required)
		return
	}

	rng, ok := validateDuration(f, "range", p.Range, time.Minute, 90*24*time.Hour)

	if p.Step == "" {
		return
	}

	step, ok2 := validateDuration(f, "step", p.Step, time.Second, 0)
	if ok && ok2 && rng/step > maxPoints {
		f.Add("step", apierr.OutOfRange)
	}
}

// validateDuration checks a duration string against lo and, if non-zero,
// hi.
func validateDuration(
	f *apierr.Fields,
	path, s string,
	lo, hi time.Duration,
) (time.Duration, bool) {
	d, err := ParseDuration(s)
	switch {
	case err != nil:
		f.Add(path, apierr.InvalidDuration)
	case d < lo || hi > 0 && d > hi:
		f.Add(path, apierr.OutOfRange)
	default:
		return d, true
	}
	return 0, false
}

// Duration returns the parsed value of a validated duration string, or 0
// if it is empty.
func Duration(s string) time.Duration {
	d, _ := ParseDuration(s)
	return d
}

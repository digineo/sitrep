package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

func TestPanelNormalize(t *testing.T) {
	assert := assert.New(t)

	all := Panel{
		ID:         "id",
		Site:       "site",
		Title:      Text{"en": " Up "},
		DataSource: "ds",
		Query:      " up ",
		Order:      3,
		Refresh:    "90s",
		Revision:   2,
		Reduce:     "max",
		Decimals:   intPtr(3),
		Unit:       Text{"en": " ms "},
		Thresholds: []Threshold{{
			Op:    "<",
			Value: 1,
			State: StateDown,
		}},
		Range:   "60m",
		Step:    "60s",
		Style:   "area",
		MinZero: true,
		Legend:  Text{"en": "{{job}}"},
	}
	common := Panel{
		ID:         "id",
		Site:       "site",
		Title:      Text{"en": "Up"},
		DataSource: "ds",
		Query:      "up",
		Order:      3,
		Refresh:    "1m30s",
		Revision:   2,
	}

	stat := all
	stat.Type = PanelStat
	stat.Normalize()
	want := common
	want.Type = PanelStat
	want.Reduce = "max"
	want.Decimals = intPtr(3)
	want.Unit = Text{"en": "ms"}
	assert.Equal(want, stat)

	status := all
	status.Type = PanelStatus
	status.Normalize()
	want = common
	want.Type = PanelStatus
	want.Reduce = "max"
	want.Thresholds = all.Thresholds
	assert.Equal(want, status)

	ts := all
	ts.Type = PanelTimeseries
	ts.Normalize()
	want = common
	want.Type = PanelTimeseries
	want.Range = "1h"
	want.Step = "1m"
	want.Style = "area"
	want.MinZero = true
	want.Decimals = intPtr(3)
	want.Unit = Text{"en": "ms"}
	want.Legend = Text{"en": "{{job}}"}
	assert.Equal(want, ts)

	defaults := Panel{Type: PanelStat}
	defaults.Normalize()
	want = Panel{
		Type:     PanelStat,
		Reduce:   "first",
		Decimals: intPtr(0),
	}
	assert.Equal(want, defaults)

	defaults = Panel{Type: PanelTimeseries}
	defaults.Normalize()
	want = Panel{
		Type:     PanelTimeseries,
		Style:    "line",
		Decimals: intPtr(2),
	}
	assert.Equal(want, defaults)
}

func TestPanelValidate(t *testing.T) {
	langs := Languages{
		Enabled: []string{"en", "de"},
		Primary: "en",
	}
	valid := func(typ string) Panel {
		p := Panel{
			Type:       typ,
			Title:      Text{"en": "Up"},
			DataSource: "ds",
			Query:      "up",
			Range:      "1h",
		}
		p.Normalize()
		return p
	}

	for _, typ := range []string{PanelStat, PanelStatus, PanelTimeseries} {
		p := valid(typ)
		require.NoError(t, p.Validate(langs, true), typ)
	}

	tests := []struct {
		name   string
		typ    string
		modify func(*Panel)
		want   map[string]string
	}{
		{"type", "gauge", func(*Panel) {}, map[string]string{"type": "invalid_value"}},
		{"title", PanelStat, func(p *Panel) { p.Title = Text{"de": "x"} }, map[string]string{"title.en": "required"}},
		{"description", PanelStat, func(p *Panel) { p.Description = Text{"de": strings.Repeat("x", 2001)} }, map[string]string{"description.de": "too_long"}},
		{"datasource", PanelStat, func(p *Panel) { p.DataSource = "" }, map[string]string{"datasource": "required"}},
		{"query", PanelStat, func(p *Panel) { p.Query = "" }, map[string]string{"query": "required"}},
		{"long query", PanelStat, func(p *Panel) { p.Query = strings.Repeat("x", 10001) }, map[string]string{"query": "too_long"}},
		{"refresh", PanelStat, func(p *Panel) { p.Refresh = "5s" }, nil},
		{"refresh too short", PanelStat, func(p *Panel) { p.Refresh = "4s" }, map[string]string{"refresh": "out_of_range"}},
		{"refresh too long", PanelStat, func(p *Panel) { p.Refresh = "1d1s" }, map[string]string{"refresh": "out_of_range"}},
		{"refresh syntax", PanelStat, func(p *Panel) { p.Refresh = "30" }, map[string]string{"refresh": "invalid_duration"}},
		{"reduce", PanelStat, func(p *Panel) { p.Reduce = "median" }, map[string]string{"reduce": "invalid_value"}},
		{"decimals", PanelStat, func(p *Panel) { p.Decimals = intPtr(7) }, map[string]string{"decimals": "out_of_range"}},
		{"negative decimals", PanelTimeseries, func(p *Panel) { p.Decimals = intPtr(-1) }, map[string]string{"decimals": "out_of_range"}},
		{"unit", PanelStat, func(p *Panel) { p.Unit = Text{"en": strings.Repeat("x", 33)} }, map[string]string{"unit.en": "too_long"}},
		{"thresholds", PanelStatus, func(p *Panel) {
			p.Thresholds = []Threshold{{Op: "<", State: StateDown}, {Op: "=", State: "ok"}}
		}, map[string]string{"thresholds[1].op": "invalid_value", "thresholds[1].state": "invalid_value"}},
		{"too many thresholds", PanelStatus, func(p *Panel) {
			p.Thresholds = make([]Threshold, 21)
			for i := range p.Thresholds {
				p.Thresholds[i] = Threshold{Op: "<", State: StateDown}
			}
		}, map[string]string{"thresholds": "too_long"}},
		{"range required", PanelTimeseries, func(p *Panel) { p.Range = "" }, map[string]string{"range": "required"}},
		{"range too short", PanelTimeseries, func(p *Panel) { p.Range = "59s" }, map[string]string{"range": "out_of_range"}},
		{"range too long", PanelTimeseries, func(p *Panel) { p.Range = "90d1s" }, map[string]string{"range": "out_of_range"}},
		{"step", PanelTimeseries, func(p *Panel) { p.Range, p.Step = "1d", "8s" }, nil},
		{"too many points", PanelTimeseries, func(p *Panel) { p.Range, p.Step = "1d", "7s" }, map[string]string{"step": "out_of_range"}},
		{"step too short", PanelTimeseries, func(p *Panel) { p.Step = "0s" }, map[string]string{"step": "out_of_range"}},
		{"style", PanelTimeseries, func(p *Panel) { p.Style = "bars" }, map[string]string{"style": "invalid_value"}},
		{"legend", PanelTimeseries, func(p *Panel) { p.Legend = Text{"en": strings.Repeat("x", 201)} }, map[string]string{"legend.en": "too_long"}},
	}
	for _, tt := range tests {
		p := valid(tt.typ)
		tt.modify(&p)
		assert.Equal(t, tt.want, fieldCodes(t, p.Validate(langs, true)), tt.name)
	}

	p := valid(PanelStat)
	p.Title = nil
	assert.NoError(t, p.Validate(langs, false), "previews need no title")
}

package process

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
)

func ptr(v float64) *float64 { return &v }

func samples(values ...float64) datasource.Result {
	// Listed in reverse label order, so that sorting is observable.
	var r datasource.Result
	for i, v := range values {
		s := datasource.Sample{
			Labels: map[string]string{"i": string(rune('a' + i))},
			Value:  v,
		}
		r.Samples = append([]datasource.Sample{s}, r.Samples...)
	}
	return r
}

func TestReduce(t *testing.T) {
	tests := []struct {
		reduce string
		result datasource.Result
		want   *float64
	}{
		{"first", samples(1, 2, 3), ptr(1)},
		{"last", samples(1, 2, 3), ptr(3)},
		{"sum", samples(1, 2, 3), ptr(6)},
		{"avg", samples(1, 2, 3), ptr(2)},
		{"min", samples(2, 1, 3), ptr(1)},
		{"max", samples(2, 3, 1), ptr(3)},
		{"sum", samples(), nil},
		{"first", datasource.Result{Scalar: ptr(7)}, ptr(7)},
		{"sum", datasource.Result{Scalar: ptr(7), Samples: samples(1).Samples}, ptr(7)},
		{"first", datasource.Result{Scalar: ptr(math.NaN())}, nil},
		{"first", samples(math.Inf(1), 2), nil},
		{"last", samples(math.Inf(1), 2), ptr(2)},
		{"sum", samples(math.NaN(), 2), nil},
		{"min", samples(2, math.NaN()), nil},
		{"max", samples(2, math.Inf(-1)), ptr(2)},
		{"min", samples(2, math.Inf(-1)), nil},
	}
	for _, tt := range tests {
		p := model.Panel{
			Type:   model.PanelStat,
			Reduce: tt.reduce,
		}
		d, err := Process(p, tt.result)
		require.NoError(t, err)
		assert.Equal(t, tt.want, d.Value, "%s %v", tt.reduce, tt.result)
	}
}

func TestThresholds(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	thresholds := []model.Threshold{
		{
			Op:    "<",
			Value: 1,
			State: model.StateDown,
		},
		{
			Op:    "<",
			Value: 5,
			State: model.StateDegraded,
		},
		{
			Op:    "==",
			Value: 5,
			State: model.StateDegraded,
		},
		{
			Op:    ">=",
			Value: 100,
			State: model.StateDown,
		},
	}
	for v, want := range map[float64]string{
		0:   model.StateDown,
		1:   model.StateDegraded,
		5:   model.StateDegraded,
		6:   model.StateOperational,
		100: model.StateDown,
	} {
		p := model.Panel{
			Type:       model.PanelStatus,
			Thresholds: thresholds,
		}
		d, err := Process(p, datasource.Result{Scalar: ptr(v)})
		require.NoError(err)
		assert.Equal(want, d.State, v)
	}

	p := model.Panel{Type: model.PanelStatus}
	d, err := Process(p, datasource.Result{Scalar: ptr(0)})
	require.NoError(err)
	assert.Equal(model.StateOperational, d.State, "no thresholds")

	p = model.Panel{
		Type:       model.PanelStatus,
		Thresholds: thresholds,
	}
	d, err = Process(p, samples())
	require.NoError(err)
	assert.Equal(model.StateUnknown, d.State, "no data")

	for _, op := range []struct {
		op   string
		hits []float64
	}{
		{"<", []float64{1}},
		{"<=", []float64{1, 2}},
		{">", []float64{3}},
		{">=", []float64{2, 3}},
		{"==", []float64{2}},
		{"!=", []float64{1, 3}},
	} {
		for _, v := range []float64{1, 2, 3} {
			th := model.Threshold{
				Op:    op.op,
				Value: 2,
			}
			assert.Equal(contains(op.hits, v), th.Matches(v), "%v %s 2", v, op.op)
		}
	}
}

func contains(s []float64, v float64) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestResultKinds(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	times := []time.Time{time.Unix(0, 0)}
	p := model.Panel{Type: model.PanelStat}
	_, err := Process(p, datasource.Result{Times: times})
	assert.ErrorIs(err, errScalarOrSamples)

	_, err = Process(model.Panel{Type: model.PanelTimeseries}, samples(1))
	assert.ErrorIs(err, errSeries)

	p = model.Panel{Type: model.PanelTimeseries}
	d, err := Process(p, datasource.Result{Times: []time.Time{}})
	require.NoError(err)
	assert.Empty(d.Series, "a range without series is no data")
}

func TestSeries(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	times := []time.Time{time.Unix(60, 0), time.Unix(120, 0), time.Unix(180, 0)}
	var r datasource.Result
	r.Times = times
	for i := range MaxSeries + 3 {
		r.Series = append(r.Series, datasource.Series{
			Labels: map[string]string{"n": string(rune('z' - i))},
			Values: []float64{float64(i), math.NaN(), math.Inf(1)},
		})
	}

	d, err := Process(model.Panel{Type: model.PanelTimeseries}, r)
	require.NoError(err)
	assert.Equal(times, d.Times)
	assert.Equal(3, d.Dropped)
	require.Len(d.Series, MaxSeries)
	labels := map[string]string{"n": string(rune('z' - MaxSeries - 2))}
	assert.Equal(labels, d.Series[0].Labels, "sorted by labels")
	values := []*float64{ptr(float64(MaxSeries + 2)), nil, nil}
	assert.Equal(values, d.Series[0].Values, "non-finite values are gaps")
}

func TestCompareLabels(t *testing.T) {
	sets := []map[string]string{
		{"a": "1"},
		{"a": "1", "b": "1"},
		{"a": "2"},
		{"b": "0"},
		{},
	}
	for i, a := range sets {
		for j, b := range sets {
			want := 0
			switch {
			case len(a) == 0 && len(b) > 0:
				want = -1
			case len(b) == 0 && len(a) > 0:
				want = 1
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			assert.Equal(t, want, compareLabels(a, b), "%v %v", a, b)
		}
	}
}

func TestNames(t *testing.T) {
	assert := assert.New(t)

	langs := model.Languages{
		Enabled: []string{"en", "de"},
		Primary: "en",
	}
	d := Data{Series: []Series{
		{Labels: map[string]string{
			"instance": "web-1",
			"job":      "api",
		}},
		{Labels: map[string]string{"instance": "web-2"}},
	}}
	p := model.Panel{
		Title: model.Text{
			"en": "Latency",
			"de": "Latenz",
		},
		Legend: model.Text{
			"en": "{{instance}} ({{ job }}) {{ missing }} {{ bad-name }}",
			"de": "  {{job}}  ",
		},
	}
	want := []string{"web-1 (api)  {{ bad-name }}", "web-2 ()  {{ bad-name }}"}
	assert.Equal(want, d.Names(p, "en", langs))
	assert.Equal(
		[]string{"api", "Latenz #2"},
		d.Names(p, "de", langs),
		"blank names fall back to the numbered title",
	)

	p.Legend = nil
	assert.Equal([]string{"Latency #1", "Latency #2"}, d.Names(p, "en", langs))

	d.Series = d.Series[:1]
	assert.Equal(
		[]string{"Latenz"},
		d.Names(p, "de", langs),
		"a single series is named like the panel",
	)
}

func TestWorst(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(model.StateOperational, Worst())
	assert.Equal(
		model.StateUnknown,
		Worst(model.StateOperational, model.StateUnknown),
	)
	assert.Equal(
		model.StateDegraded,
		Worst(model.StateUnknown, model.StateDegraded, model.StateOperational),
	)
	assert.Equal(model.StateDown, Worst(model.StateDown, model.StateDegraded))
}

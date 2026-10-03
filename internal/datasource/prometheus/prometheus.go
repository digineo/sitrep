// Package prometheus is the data source type for Prometheus and compatible
// backends.
package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
)

func init() {
	datasource.Register("prometheus", Type{})
}

// Type is the Prometheus data source type.
type Type struct{}

// Fields describes the configuration: the base URL, authentication and the
// request timeout.
func (Type) Fields() []datasource.Field {
	return []datasource.Field{
		{
			Name:     "url",
			Kind:     datasource.KindURL,
			Required: true,
		},
		{
			Name:    "auth",
			Kind:    datasource.KindSelect,
			Default: "none",
			Options: []string{"none", "basic", "bearer"},
		},
		{
			Name:     "username",
			Kind:     datasource.KindText,
			Required: true,
			When: &datasource.Condition{
				Field: "auth",
				Value: "basic",
			},
		},
		{
			Name: "password",
			Kind: datasource.KindSecret,
			When: &datasource.Condition{
				Field: "auth",
				Value: "basic",
			},
		},
		{
			Name:     "token",
			Kind:     datasource.KindSecret,
			Required: true,
			When: &datasource.Condition{
				Field: "auth",
				Value: "bearer",
			},
		},
		{
			Name:    "timeout",
			Kind:    datasource.KindDuration,
			Default: "10s",
			Min:     "1s",
			Max:     "2m",
		},
	}
}

// PanelTypes returns every panel type.
func (Type) PanelTypes() []string {
	return []string{model.PanelStat, model.PanelStatus, model.PanelTimeseries}
}

// Editor selects the PromQL editor.
func (Type) Editor() string { return "promql" }

// Summary returns the URL.
func (Type) Summary(cfg datasource.Config) string { return cfg["url"] }

// Test runs the query "1" and reports the round-trip time.
func (Type) Test(ctx context.Context, cfg datasource.Config) (string, error) {
	start := time.Now()
	_, _, err := call(ctx, cfg, "/api/v1/query", url.Values{"query": {"1"}})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d ms", time.Since(start).Milliseconds()), nil
}

// client never follows redirects, so that credentials cannot be forwarded
// to another host. Proxy environment variables are honored.
var client = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var (
	errInstant  = errors.New("the query must return a scalar or an instant vector")
	errRange    = errors.New("the query must return a range vector")
	errTooLarge = fmt.Errorf(
		"the response is larger than %d bytes",
		datasource.MaxResponse,
	)
)

// send sends a request to the API with the configured authentication and
// returns the response with its body read. Redirects and bodies over
// datasource.MaxResponse are errors. ctx should carry the timeout.
func send(
	ctx context.Context,
	cfg datasource.Config,
	method, path, contentType string,
	body io.Reader,
) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, cfg["url"]+path, body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	switch cfg["auth"] {
	case "basic":
		req.SetBasicAuth(cfg["username"], cfg["password"])
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+cfg["token"])
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return nil, nil, fmt.Errorf("redirect not followed: HTTP %s", res.Status)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, datasource.MaxResponse+1))
	if err != nil {
		return nil, nil, err
	}

	if len(data) > datasource.MaxResponse {
		return nil, nil, errTooLarge
	}
	return res, data, nil
}

// call posts a form to an API endpoint and returns the "data" of a
// successful answer and the size of the response, or the error Prometheus
// reports.
func call(
	ctx context.Context,
	cfg datasource.Config,
	path string,
	form url.Values,
) (json.RawMessage, int, error) {
	ctx, cancel := context.WithTimeout(ctx, model.Duration(cfg["timeout"]))
	defer cancel()

	res, body, err := send(
		ctx,
		cfg,
		http.MethodPost,
		path,
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, 0, err
	}

	var r struct {
		Status    string          `json:"status"`
		Data      json.RawMessage `json:"data"`
		ErrorType string          `json:"errorType"`
		Error     string          `json:"error"`
	}
	err = json.Unmarshal(body, &r)
	switch {
	case err == nil && r.Status == "error":
		return nil, 0, fmt.Errorf("%s: %s", r.ErrorType, r.Error)
	case res.StatusCode != http.StatusOK:
		return nil, 0, fmt.Errorf("HTTP %s", res.Status)
	case err != nil || r.Status != "success":
		return nil, 0, errors.New("invalid response")
	}
	return r.Data, len(body), nil
}

// point is a sample as Prometheus encodes it: [<unix time>, "<value>"].
type point struct {
	t float64
	v float64
}

func (p *point) UnmarshalJSON(b []byte) error {
	var raw [2]json.RawMessage
	var v string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if err := json.Unmarshal(raw[0], &p.t); err != nil {
		return err
	}

	if err := json.Unmarshal(raw[1], &v); err != nil {
		return err
	}

	var err error
	p.v, err = strconv.ParseFloat(v, 64)
	return err
}

type queryData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

// Evaluate runs an instant query for stat and status panels and a range
// query for timeseries panels.
func (Type) Evaluate(
	ctx context.Context,
	cfg datasource.Config,
	p model.Panel,
	now time.Time,
) (datasource.Result, error) {
	if p.Type == model.PanelTimeseries {
		return queryRange(ctx, cfg, p, now)
	}

	raw, size, err := call(ctx, cfg, "/api/v1/query", url.Values{
		"query": {p.Query},
		"time":  {unix(now)},
	})
	if err != nil {
		return datasource.Result{}, err
	}

	var d queryData
	if err := json.Unmarshal(raw, &d); err != nil {
		return datasource.Result{}, errors.New("invalid response")
	}

	r := datasource.Result{Bytes: size}
	switch d.ResultType {
	case "scalar":
		var pt point
		err = json.Unmarshal(d.Result, &pt)
		r.Scalar = &pt.v
	case "vector":
		var vector []struct {
			Metric map[string]string `json:"metric"`
			Value  point             `json:"value"`
		}
		err = json.Unmarshal(d.Result, &vector)
		r.Samples = make([]datasource.Sample, len(vector))
		for i, s := range vector {
			r.Samples[i] = datasource.Sample{
				Labels: s.Metric,
				Value:  s.Value.v,
			}
		}
	default:
		return datasource.Result{}, errInstant
	}
	if err != nil {
		return datasource.Result{}, errors.New("invalid response")
	}
	return r, nil
}

// stepOf returns the step of a range query: the configured one, or 1/240
// of the range rounded to whole seconds, at least 1s.
func stepOf(p model.Panel) time.Duration {
	if p.Step != "" {
		return model.Duration(p.Step)
	}
	return max(time.Second, (model.Duration(p.Range) / 240).Round(time.Second))
}

// window returns start and end of a range query in Unix seconds. The end
// is aligned to the step, so that all series share the same times.
func window(p model.Panel, now time.Time) (start, end, step int64) {
	step = int64(stepOf(p) / time.Second)
	end = now.Unix() / step * step
	return end - int64(model.Duration(p.Range)/time.Second), end, step
}

// queryRange runs a range query and puts each series on the window's time
// axis. Times without a value are NaN.
func queryRange(
	ctx context.Context,
	cfg datasource.Config,
	p model.Panel,
	now time.Time,
) (datasource.Result, error) {
	start, end, step := window(p, now)
	raw, size, err := call(ctx, cfg, "/api/v1/query_range", url.Values{
		"query": {p.Query},
		"start": {strconv.FormatInt(start, 10)},
		"end":   {strconv.FormatInt(end, 10)},
		"step":  {strconv.FormatInt(step, 10)},
	})
	if err != nil {
		return datasource.Result{}, err
	}

	var d queryData
	var matrix []struct {
		Metric map[string]string `json:"metric"`
		Values []point           `json:"values"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return datasource.Result{}, errors.New("invalid response")
	}

	if d.ResultType != "matrix" {
		return datasource.Result{}, errRange
	}

	if err := json.Unmarshal(d.Result, &matrix); err != nil {
		return datasource.Result{}, errors.New("invalid response")
	}

	n := (end-start)/step + 1
	r := datasource.Result{
		Times:  make([]time.Time, n),
		Series: make([]datasource.Series, len(matrix)),
		Bytes:  size,
	}
	for i := range n {
		r.Times[i] = time.Unix(start+i*step, 0).UTC()
	}

	for i, m := range matrix {
		values := make([]float64, n)
		for j := range values {
			values[j] = math.NaN()
		}

		for _, pt := range m.Values {
			offset := int64(math.Round(pt.t)) - start
			if offset >= 0 && offset%step == 0 && offset/step < n {
				values[offset/step] = pt.v
			}
		}

		r.Series[i] = datasource.Series{
			Labels: m.Metric,
			Values: values,
		}
	}
	return r, nil
}

func unix(t time.Time) string {
	return strconv.FormatFloat(float64(t.UnixMilli())/1000, 'f', -1, 64)
}

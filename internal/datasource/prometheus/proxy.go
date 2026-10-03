package prometheus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
)

// discovery returns the methods the discovery proxy forwards for an API
// path, and the path to request upstream. The endpoints are read-only and
// serve PromQL autocompletion.
func discovery(path string) ([]string, string) {
	both := []string{http.MethodGet, http.MethodPost}
	switch path {
	case "/api/v1/labels", "/api/v1/series":
		return both, path
	case "/api/v1/metadata", "/api/v1/status/flags":
		return both[:1], path
	}

	name, prefixed := strings.CutPrefix(path, "/api/v1/label/")
	name, suffixed := strings.CutSuffix(name, "/values")
	if !prefixed || !suffixed || name == "" || name == "." || name == ".." ||
		strings.Contains(name, "/") {
		return nil, ""
	}
	return both[:1], "/api/v1/label/" + url.PathEscape(name) + "/values"
}

// ServeAdmin is the discovery proxy. It forwards allowed requests with the
// configured authentication and none of the inbound credentials or proxy
// headers. Only successful answers are passed on, as JSON and without
// upstream headers; anything else is a bad gateway, so that an upstream 401
// is never taken for the end of the admin's session, and an upstream HTML
// page is never rendered on the admin origin.
func (Type) ServeAdmin(
	w http.ResponseWriter,
	r *http.Request,
	cfg datasource.Config,
	path string,
) error {
	methods, upstream := discovery(path)
	switch {
	case methods == nil:
		return apierr.New(http.StatusNotFound, apierr.NotFound)
	case !slices.Contains(methods, r.Method):
		w.Header().Set("Allow", strings.Join(methods, ", "))
		return apierr.New(http.StatusMethodNotAllowed, apierr.MethodNotAllowed)
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxBody))
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return apierr.New(http.StatusRequestEntityTooLarge, apierr.TooLarge)
	} else if err != nil {
		return err
	}

	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}

	ctx, cancel := context.WithTimeout(r.Context(), model.Duration(cfg["timeout"]))
	defer cancel()

	var contentType string
	if r.Method == http.MethodPost {
		contentType = r.Header.Get("Content-Type")
	}

	res, data, err := send(
		ctx,
		cfg,
		r.Method,
		upstream,
		contentType,
		bytes.NewReader(body),
	)
	if err == nil && res.StatusCode/100 != 2 {
		err = fmt.Errorf("HTTP %s", res.Status)
	}
	if err != nil {
		e := apierr.New(http.StatusBadGateway, apierr.BadGateway)
		return errors.Join(e, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(data)
	return nil
}

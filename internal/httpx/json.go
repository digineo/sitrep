package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/i18n"
)

// MaxBody is the size limit of request bodies.
const MaxBody = 1 << 20

// ReadJSON decodes the request body into v. Bodies over MaxBody, unknown
// fields and trailing data are rejected.
func ReadJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil {
		if _, err = dec.Token(); err == io.EOF {
			return nil
		}
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return apierr.New(http.StatusRequestEntityTooLarge, apierr.TooLarge)
	}
	return apierr.New(http.StatusBadRequest, apierr.InvalidJSON)
}

// WriteJSON answers with v as JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Fields  []apierr.Field `json:"fields,omitempty"`
}

// WriteError answers with err if it is an *apierr.Error. Any other error is
// logged with the request ID and answered with a generic 500.
func WriteError(
	w http.ResponseWriter,
	r *http.Request,
	log xlog.Logger,
	err error,
) {
	e, ok := errors.AsType[*apierr.Error](err)
	if !ok {
		log.Error("internal error",
			slog.String("request_id", RequestID(r)),
			xlog.Error(err))
		e = apierr.New(http.StatusInternalServerError, apierr.Internal)
	}
	WriteJSON(w, e.Status, map[string]errorBody{"error": {
		Code:    e.Code,
		Message: i18n.Get(i18n.Reference).T("error."+e.Code, nil),
		Fields:  e.Fields,
	}})
}

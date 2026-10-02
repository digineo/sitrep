// Package apierr defines the machine-readable errors of the HTTP APIs.
package apierr

import "net/http"

// Error is an API error with a stable code. Validation errors list each
// failing field.
type Error struct {
	Status int     `json:"-"`
	Code   string  `json:"code"`
	Fields []Field `json:"fields,omitempty"`
}

// Field names a failing field by its path, e.g. "languages.primary".
type Field struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

func (e *Error) Error() string { return e.Code }

// New returns an error with the given HTTP status and code.
func New(status int, code string) *Error {
	return &Error{
		Status: status,
		Code:   code,
	}
}

// Fields collects field errors during validation.
type Fields []Field

// Add records a failing field.
func (f *Fields) Add(path, code string) {
	*f = append(*f, Field{
		Path: path,
		Code: code,
	})
}

// Err returns a validation error listing the fields, or nil if there are none.
func (f Fields) Err() error {
	if len(f) == 0 {
		return nil
	}
	return &Error{
		Status: http.StatusBadRequest,
		Code:   Invalid,
		Fields: f,
	}
}

var codes []string

func code(c string) string {
	codes = append(codes, c)
	return c
}

// Codes returns every defined error code.
func Codes() []string { return codes }

// Error codes. Each one needs a message in every catalog.
var (
	Internal             = code("internal")
	Invalid              = code("invalid")
	InvalidJSON          = code("invalid_json")
	TooLarge             = code("too_large")
	NotFound             = code("not_found")
	Unauthorized         = code("unauthorized")
	Forbidden            = code("forbidden")
	UnsupportedMediaType = code("unsupported_media_type")
	Throttled            = code("throttled")
	InvalidCredentials   = code("invalid_credentials")
	Required             = code("required")
	InvalidValue         = code("invalid_value")
	UnsupportedLanguage  = code("unsupported_language")
	Duplicate            = code("duplicate")
	PrimaryNotEnabled    = code("primary_not_enabled")
	RouteConflict        = code("route_conflict")
)

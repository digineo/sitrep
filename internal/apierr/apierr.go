// Package apierr defines the machine-readable errors of the HTTP APIs.
package apierr

import "net/http"

// Error is an API error with a stable code. Validation errors list each
// failing field. Details carry data a client needs to explain the error.
type Error struct {
	Status  int     `json:"-"`
	Code    string  `json:"code"`
	Fields  []Field `json:"fields,omitempty"`
	Details any     `json:"details,omitempty"`
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
	MethodNotAllowed     = code("method_not_allowed")
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
	TooLong              = code("too_long")
	InvalidSlug          = code("invalid_slug")
	SlugReserved         = code("slug_reserved")
	InvalidDomain        = code("invalid_domain")
	DomainReserved       = code("domain_reserved")
	InvalidTimezone      = code("invalid_timezone")
	InvalidDuration      = code("invalid_duration")
	InvalidURL           = code("invalid_url")
	OutOfRange           = code("out_of_range")
	NameTaken            = code("name_taken")
	SecretKeyMissing     = code("secret_key_missing")
	DataSourceInUse      = code("datasource_in_use")
	DataSourceUnusable   = code("datasource_unusable")
	UnsupportedType      = code("datasource_unsupported_type")
	BadGateway           = code("bad_gateway")
	FirstUpdateMustOpen  = code("first_update_must_open")
	StatusOrSeverity     = code("status_or_severity_required")
	InvalidOrigin        = code("invalid_origin")
	InvalidLink          = code("invalid_link")
	InvalidSVG           = code("invalid_svg")
	SVGTooLarge          = code("svg_too_large")
	SiteUnavailable      = code("site_unavailable")
	InvalidYAML          = code("invalid_yaml")
	UnsupportedVersion   = code("unsupported_version")
)

// Package config reads the server configuration from environment variables
// and dotenv files.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/digineo/sitrep/internal/model"
)

// Env reads configuration variables. It collects every error, so that all
// of them can be reported together, and the effective values for the
// startup log, with secrets redacted.
type Env struct {
	lookup func(string) (string, bool)
	errs   []error
	attrs  []slog.Attr
}

// NewEnv returns an Env reading variables with lookup.
func NewEnv(lookup func(string) (string, bool)) *Env {
	return &Env{lookup: lookup}
}

// Errorf records an error for the variable name.
func (e *Env) Errorf(name, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	e.errs = append(e.errs, fmt.Errorf("%s: %s", name, msg))
}

// Err returns all recorded errors, or nil.
func (e *Env) Err() error {
	return errors.Join(e.errs...)
}

// LogAttrs returns the effective values read so far, secrets redacted.
func (e *Env) LogAttrs() []slog.Attr {
	return e.attrs
}

func (e *Env) log(name string, value any) {
	e.attrs = append(e.attrs, slog.Any(name, value))
}

// String returns the variable's value, or def if it is unset. A set but
// empty value is an error.
func (e *Env) String(name, def string) string {
	v, ok := e.lookup(name)
	if !ok {
		v = def
	} else if v == "" {
		e.Errorf(name, "must not be empty")
	}
	e.log(name, v)
	return v
}

// Required returns the variable's value, which must be set and not empty.
func (e *Env) Required(name string) string {
	v, _ := e.lookup(name)
	if v == "" {
		e.Errorf(name, "is required")
	}
	e.log(name, v)
	return v
}

// Secret returns the variable's value, or "" if it is unset. The startup
// log only says whether it is set.
func (e *Env) Secret(name string) string {
	v, _ := e.lookup(name)
	e.log(name, redacted(v != ""))
	return v
}

// URL returns the variable's value as absolute http(s) URL. The startup
// log omits embedded credentials.
func (e *Env) URL(name string, required bool) *url.URL {
	v, _ := e.lookup(name)
	if v == "" {
		if required {
			e.Errorf(name, "is required")
		}

		e.log(name, "")
		return nil
	}

	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		e.Errorf(name, "must be an absolute http or https URL")
		e.log(name, redacted(true))
		return nil
	}

	e.log(name, u.Redacted())
	return u
}

// Duration returns the variable's value as duration between lo and hi, or
// def if it is unset.
func (e *Env) Duration(name string, def, lo, hi time.Duration) time.Duration {
	v, ok := e.lookup(name)
	if ok {
		d, err := model.ParseDuration(v)
		switch {
		case err != nil:
			e.Errorf(name, "%v", err)
		case d < lo || d > hi:
			e.Errorf(
				name,
				"must be between %s and %s",
				model.FormatDuration(lo),
				model.FormatDuration(hi),
			)
		default:
			def = d
		}
	}
	e.log(name, model.FormatDuration(def))
	return def
}

// Enum returns the variable's value, which must be one of allowed, or def
// if it is unset.
func (e *Env) Enum(name, def string, allowed ...string) string {
	v, ok := e.lookup(name)
	if !ok {
		v = def
	} else if !slices.Contains(allowed, v) {
		e.Errorf(name, "must be one of %s", strings.Join(allowed, ", "))
	}
	e.log(name, v)
	return v
}

// Key returns the variable's value as base64-encoded 32-byte key, or nil
// if it is unset.
func (e *Env) Key(name string) []byte {
	v, _ := e.lookup(name)
	e.log(name, redacted(v != ""))
	if v == "" {
		return nil
	}

	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(key) != 32 {
		e.Errorf(
			name,
			"must be a base64-encoded 32-byte key, "+
				"e.g. from: openssl rand -base64 32",
		)
		return nil
	}
	return key
}

func redacted(set bool) string {
	if set {
		return "(redacted)"
	}
	return "(unset)"
}

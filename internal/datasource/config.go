package datasource

import (
	"errors"
	"maps"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/model"
)

// Kind is the kind of a configuration field.
type Kind string

// Field kinds.
const (
	KindText Kind = "text"
	// an absolute http(s) URL without userinfo, query and fragment
	KindURL      Kind = "url"
	KindSecret   Kind = "secret"   // stored encrypted, never returned
	KindDuration Kind = "duration" // within Min and Max
	KindSelect   Kind = "select"   // one of Options
	KindBool     Kind = "bool"     // "true" or "false"
)

const (
	maxName  = 100
	maxValue = 2000
)

// Field describes a configuration field. Labels and help texts come from
// the catalogs, keyed datasource.<type>.<field>.
type Field struct {
	Name     string     `json:"name"`
	Kind     Kind       `json:"kind"`
	Required bool       `json:"required,omitempty"`
	Default  string     `json:"default,omitempty"`
	Options  []string   `json:"options,omitempty"`
	Min      string     `json:"min,omitempty"`
	Max      string     `json:"max,omitempty"`
	When     *Condition `json:"when,omitempty"`
}

// Condition shows a field only while an earlier field has a value.
type Condition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// ErrUnusable means that a data source's secrets cannot be decrypted.
var ErrUnusable = errors.New(
	"a secret cannot be decrypted: " +
		"the secret key is missing or has changed; enter the secret again",
)

// Apply validates a submitted configuration and writes it to ds, whose ID
// and Type must be set. For secret fields, an absent key keeps the stored
// value, an empty value clears it, and new values are sealed with key.
// Fields hidden by their condition are dropped. Field errors are added to f
// with the path "config.<field>"; ds is only changed without them.
func Apply(
	f *apierr.Fields,
	ds *model.DataSource,
	in map[string]string,
	key []byte,
) error {
	errs := len(*f)
	fields := Get(ds.Type).Fields()
	for _, k := range slices.Sorted(maps.Keys(in)) {
		known := func(fd Field) bool { return fd.Name == k }
		if !slices.ContainsFunc(fields, known) {
			f.Add("config."+k, apierr.InvalidValue)
		}
	}

	config := map[string]string{}
	secrets := map[string][]byte{}
	for _, fd := range fields {
		if fd.When != nil && config[fd.When.Field] != fd.When.Value {
			continue
		}

		path := "config." + fd.Name
		v, submitted := in[fd.Name]
		if fd.Kind == KindSecret {
			switch {
			case !submitted && ds.Secrets[fd.Name] != nil:
				secrets[fd.Name] = ds.Secrets[fd.Name]
			case !submitted || v == "":
				if fd.Required {
					f.Add(path, apierr.Required)
				}
			case key == nil:
				f.Add(path, apierr.SecretKeyMissing)
			default:
				sealed, err := seal(key, ds.ID, v)
				if err != nil {
					return err
				}

				secrets[fd.Name] = sealed
			}
			continue
		}

		if v = strings.TrimSpace(v); v == "" {
			v = fd.Default
		}
		if v == "" {
			if fd.Required {
				f.Add(path, apierr.Required)
			}
			continue
		}

		if v, code := fd.normalize(v); code != "" {
			f.Add(path, code)
		} else {
			config[fd.Name] = v
		}
	}

	if len(*f) == errs {
		ds.Config, ds.Secrets = config, secrets
	}
	return nil
}

// ValidateName trims a data source name and checks its length.
func ValidateName(f *apierr.Fields, name string) string {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		f.Add("name", apierr.Required)
	case utf8.RuneCountInString(name) > maxName:
		f.Add("name", apierr.TooLong)
	}
	return name
}

// normalize returns the canonical form of a non-empty value, or an error
// code.
func (fd Field) normalize(v string) (string, string) {
	if utf8.RuneCountInString(v) > maxValue {
		return "", apierr.TooLong
	}

	switch fd.Kind {
	case KindURL:
		u, err := url.Parse(v)
		if err != nil || u.Scheme != "http" && u.Scheme != "https" ||
			u.Host == "" || u.Opaque != "" || u.User != nil ||
			u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return "", apierr.InvalidURL
		}
		return strings.TrimRight(v, "/"), ""
	case KindDuration:
		d, err := model.ParseDuration(v)
		if err != nil {
			return "", apierr.InvalidDuration
		}

		if d < model.Duration(fd.Min) ||
			fd.Max != "" && d > model.Duration(fd.Max) {
			return "", apierr.OutOfRange
		}
		return model.FormatDuration(d), ""
	case KindSelect:
		if !slices.Contains(fd.Options, v) {
			return "", apierr.InvalidValue
		}
	case KindBool:
		if v != "true" && v != "false" {
			return "", apierr.InvalidValue
		}
	}
	return v, ""
}

// Open returns the configuration of ds with its secrets decrypted with
// key. It fails with ErrUnusable if a secret cannot be decrypted.
func Open(ds model.DataSource, key []byte) (Config, error) {
	cfg := Config(maps.Clone(ds.Config))
	if cfg == nil {
		cfg = Config{}
	}

	for name, sealed := range ds.Secrets {
		v, err := unseal(key, ds.ID, sealed)
		if err != nil {
			return nil, ErrUnusable
		}

		cfg[name] = v
	}
	return cfg, nil
}

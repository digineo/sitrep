// Package i18n provides the translation catalogs and language negotiation.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/digineo/sitrep/locales"
)

// Reference is the reference catalog: every other catalog has its keys.
const Reference = "en"

var codePattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

// ValidCode reports whether s is shaped like a language code, e.g. "de" or
// "pt-br".
func ValidCode(s string) bool {
	return codePattern.MatchString(s)
}

// Catalog holds the messages of one language.
type Catalog struct {
	Lang    string
	Name    string // the language's own name
	Imprint string // URL slug of the imprint
	Privacy string // URL slug of the privacy statement

	messages map[string]string
}

var catalogs, supported = mustLoad(locales.FS)

func mustLoad(fsys fs.FS) (map[string]*Catalog, []string) {
	c, langs, err := load(fsys)
	if err != nil {
		panic(err)
	}
	return c, langs
}

func load(fsys fs.FS) (map[string]*Catalog, []string, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, nil, err
	}

	all := make(map[string]*Catalog, len(names))
	langs := make([]string, 0, len(names))
	for _, name := range names {
		lang := strings.TrimSuffix(name, ".json")
		if !ValidCode(lang) {
			return nil, nil, fmt.Errorf("catalog %s: invalid language code", name)
		}

		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, nil, err
		}

		var tree map[string]any
		if err := json.Unmarshal(raw, &tree); err != nil {
			return nil, nil, fmt.Errorf("catalog %s: %w", name, err)
		}

		messages := map[string]string{}
		if err := flatten(messages, "", tree); err != nil {
			return nil, nil, fmt.Errorf("catalog %s: %w", name, err)
		}

		all[lang] = &Catalog{
			Lang:     lang,
			Name:     messages["language.name"],
			Imprint:  messages["legal.imprint.slug"],
			Privacy:  messages["legal.privacy.slug"],
			messages: messages,
		}
		langs = append(langs, lang)
	}

	if all[Reference] == nil {
		return nil, nil, fmt.Errorf(
			"reference catalog %s.json is missing",
			Reference,
		)
	}
	return all, langs, nil
}

func flatten(dst map[string]string, prefix string, tree map[string]any) error {
	for k, v := range tree {
		key := prefix + k
		switch v := v.(type) {
		case string:
			dst[key] = v
		case map[string]any:
			if err := flatten(dst, key+".", v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: messages must be strings or objects", key)
		}
	}
	return nil
}

// Supported returns the codes of all languages with a catalog, sorted.
func Supported() []string {
	return slices.Clone(supported)
}

// IsSupported reports whether lang has a catalog.
func IsSupported(lang string) bool {
	return catalogs[lang] != nil
}

// Get returns the catalog of lang, or nil if it is not supported.
func Get(lang string) *Catalog {
	return catalogs[lang]
}

var placeholderPattern = regexp.MustCompile(`\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}`)

// T returns the message for key with its {placeholders} replaced by params.
// A missing key returns the key itself.
func (c *Catalog) T(key string, params map[string]string) string {
	msg, ok := c.messages[key]
	if !ok {
		return key
	}
	return placeholderPattern.ReplaceAllStringFunc(msg, func(m string) string {
		if v, ok := params[placeholderPattern.FindStringSubmatch(m)[1]]; ok {
			return v
		}
		return m
	})
}

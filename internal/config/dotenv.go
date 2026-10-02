package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

var dotenvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ReadDotenv reads dotenv files. A value from an earlier file wins over one
// from a later file. Missing files are skipped.
func ReadDotenv(paths ...string) (map[string]string, error) {
	vars := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}

		parsed, err := parseDotenv(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		for k, v := range parsed {
			if _, ok := vars[k]; !ok {
				vars[k] = v
			}
		}
	}
	return vars, nil
}

// parseDotenv parses lines of KEY=VALUE, optionally prefixed by "export".
// Values may be wrapped in single or double quotes, which are removed
// without processing escapes. Unquoted values end at " #". Blank lines and
// lines starting with # are skipped. Errors name the line, never its content.
func parseDotenv(data string) (map[string]string, error) {
	vars := map[string]string{}
	for i, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !dotenvKey.MatchString(key) {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", i+1)
		}

		if value != "" && (value[0] == '"' || value[0] == '\'') {
			if len(value) < 2 || value[len(value)-1] != value[0] {
				return nil, fmt.Errorf("line %d: unterminated quote", i+1)
			}

			value = value[1 : len(value)-1]
		} else if before, _, found := strings.Cut(value, " #"); found {
			value = strings.TrimSpace(before)
		}

		vars[key] = value
	}
	return vars, nil
}

// Lookup returns a lookup function that prefers the real environment over
// the dotenv variables.
func Lookup(dotenv map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if v, ok := os.LookupEnv(name); ok {
			return v, true
		}
		v, ok := dotenv[name]
		return v, ok
	}
}

//go:build !dev

package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/digineo/sitrep/frontend"
)

// assets serves the embedded frontend build.
type assets struct {
	fs       fs.FS
	manifest map[string]manifestChunk
}

// manifestChunk is an entry of Vite's build manifest.
type manifestChunk struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
}

func newAssets() (*assets, error) {
	dist, err := fs.Sub(frontend.Dist, "dist/app")
	if err != nil {
		return nil, err
	}

	raw, err := fs.ReadFile(dist, ".vite/manifest.json")
	if err != nil {
		return nil, errors.New(
			"the embedded frontend build is missing: " +
				"build the binary with make build",
		)
	}

	a := &assets{fs: dist}
	return a, json.Unmarshal(raw, &a.manifest)
}

// entry returns the assets of an entry point: "public" or "admin".
func (a *assets) entry(name string) shellAssets {
	chunk := a.manifest["src/"+name+"/main.ts"]
	var styles []string
	var collect func(c manifestChunk)
	// Styles of imported chunks come first, so that an entry's own styles
	// win over shared ones.
	collect = func(c manifestChunk) {
		for _, imp := range c.Imports {
			collect(a.manifest[imp])
		}
		for _, css := range c.CSS {
			if !slices.Contains(styles, "/assets/"+css) {
				styles = append(styles, "/assets/"+css)
			}
		}
	}

	collect(chunk)
	return shellAssets{
		Scripts: []string{"/assets/" + chunk.File},
		Styles:  styles,
	}
}

// ServeHTTP serves a hashed asset below /assets/ with immutable caching.
func (a *assets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	fi, err := fs.Stat(a.fs, name)
	if err != nil || fi.IsDir() || strings.Contains("/"+name, "/.") {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFileFS(w, r, a.fs, name)
}

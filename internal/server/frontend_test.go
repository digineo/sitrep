//go:build !dev

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func testAssets() *assets {
	return &assets{
		fs: fstest.MapFS{
			"admin-abc.js":        {Data: []byte("admin")},
			"public-abc.js":       {Data: []byte("public")},
			"shared-abc.css":      {Data: []byte("css")},
			".vite/manifest.json": {Data: []byte("{}")},
			"nested/file.js":      {Data: []byte("x")},
		},
		manifest: map[string]manifestChunk{
			"src/admin/main.ts": {
				File:    "admin-abc.js",
				Imports: []string{"_shared.js"},
			},
			"src/public/main.ts": {
				File:    "public-abc.js",
				CSS:     []string{"public-abc.css"},
				Imports: []string{"_shared.js"},
			},
			"_shared.js": {
				File: "shared-abc.js",
				CSS:  []string{"shared-abc.css", "public-abc.css"},
			},
		},
	}
}

func TestAssetsEntry(t *testing.T) {
	a := testAssets()
	want := shellAssets{
		Scripts: []string{"/assets/public-abc.js"},
		Styles:  []string{"/assets/shared-abc.css", "/assets/public-abc.css"},
	}
	assert.Equal(t, want, a.entry("public"))

	styles := []string{"/assets/shared-abc.css", "/assets/public-abc.css"}
	assert.Equal(t, styles, a.entry("admin").Styles)
}

func TestAssetsServe(t *testing.T) {
	assert := assert.New(t)

	a := testAssets()
	serve := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	w := serve("/assets/admin-abc.js")
	assert.Equal(http.StatusOK, w.Code)
	cache := w.Header().Get("Cache-Control")
	assert.Equal("public, max-age=31536000, immutable", cache)
	assert.Equal("admin", w.Body.String())

	for _, path := range []string{
		"/assets/",
		"/assets/nested",
		"/assets/nested/",
		"/assets/missing.js",
		"/assets/.vite/manifest.json",
	} {
		w := serve(path)
		assert.Equal(http.StatusNotFound, w.Code, path)
		assert.Empty(w.Header().Get("Cache-Control"), path)
	}
}

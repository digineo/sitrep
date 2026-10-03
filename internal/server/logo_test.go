package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/model"
)

const testLogo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"></rect></svg>`

func TestBranding(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t)
	id := f.createSite("shop", "en")
	get := f.admin(http.MethodGet, "/api/admin/sites/"+id, nil)
	site := decode[model.Site](t, get, http.StatusOK)
	site.BrandColor = "#FFCC00"
	site.Logo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1" onload="alert(1)"><rect width="1" height="1"/><script>alert(1)</script></svg>`
	put := f.admin(http.MethodPut, "/api/admin/sites/"+id, site)
	saved := decode[model.Site](t, put, http.StatusOK)
	assert.Equal("#ffcc00", saved.BrandColor)
	assert.Equal(testLogo, saved.Logo, "sanitized on save")

	p, _ := f.publicPayload(id, "")
	assert.Equal("#ffcc00", p.Site.BrandColor)
	pattern := `^/api/public/sites/` + id + `/logo/[0-9a-f]{16}\.svg$`
	require.Regexp(pattern, p.Site.Logo)

	w := f.get("other.example", p.Site.Logo)
	require.Equal(http.StatusOK, w.Code)
	assert.Equal(testLogo, w.Body.String())
	h := w.Header()
	assert.Equal("image/svg+xml", h.Get("Content-Type"))
	csp := "default-src 'none'; style-src 'unsafe-inline'; sandbox"
	assert.Equal(csp, h.Get("Content-Security-Policy"))
	assert.Equal("nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal("public, max-age=31536000, immutable", h.Get("Cache-Control"))

	other := "/api/public/sites/" + id + "/logo/0123456789abcdef.svg"
	w = f.get("status.example.com", other)
	assert.Equal(http.StatusNotFound, w.Code, "another hash")

	saved.Logo = ""
	f.admin(http.MethodPut, "/api/admin/sites/"+id, saved)
	assert.Equal(http.StatusNotFound, f.get("status.example.com", p.Site.Logo).Code)
	p, _ = f.publicPayload(id, "")
	assert.Empty(p.Site.Logo)
}

func TestSanitizeSVG(t *testing.T) {
	f := newFixture(t)
	body := obj{"svg": `<svg xmlns="http://www.w3.org/2000/svg"><a href="https://evil.example"/></svg>`}
	w := f.admin(http.MethodPost, "/api/admin/svg", body)
	res := decode[map[string]string](t, w, http.StatusOK)
	assert.Equal(t, `<svg xmlns="http://www.w3.org/2000/svg"></svg>`, res["svg"])

	w = f.admin(http.MethodPost, "/api/admin/svg", obj{"svg": "<html/>"})
	codes := fieldCodes(t, w, http.StatusBadRequest)
	assert.Equal(t, map[string]string{"svg": "invalid_svg"}, codes)
}

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidSlug(t *testing.T) {
	for _, s := range []string{
		"a", "status", "my-site", "a1-b2", strings.Repeat("a", 63),
	} {
		assert.True(t, ValidSlug(s), s)
	}
	for _, s := range []string{
		"", "-a", "a-", "a--b", "A", "a_b", "a.b", "ä", strings.Repeat("a", 64),
	} {
		assert.False(t, ValidSlug(s), s)
	}
}

func TestValidDomain(t *testing.T) {
	// 253 characters
	long := strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 61)
	for _, s := range []string{
		"example.com", "status.example.co.uk", "a-b.example",
		"sitrep.localhost", long,
	} {
		assert.True(t, ValidDomain(s), s)
	}

	for _, s := range []string{
		"", "localhost", "Example.com", "example.com.", "example.com:443",
		"*.example.com", "-a.example.com", "a-.example.com", "a..example",
		"exa_mple.com", "bücher.de",
		strings.Repeat("a", 64) + ".com", long + "a",
	} {
		assert.False(t, ValidDomain(s), s)
	}
}

func TestNormalizeOrigin(t *testing.T) {
	for in, want := range map[string]string{
		"https://Example.com":      "https://example.com",
		"https://example.com/":     "https://example.com",
		"https://example.com:443":  "https://example.com",
		"http://example.com:80":    "http://example.com",
		"http://example.com:443":   "http://example.com:443",
		"https://example.com:8443": "https://example.com:8443",
		"http://[::1]:8080":        "http://[::1]:8080",
	} {
		got, ok := NormalizeOrigin(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{
		"", "null", "example.com", "ftp://example.com", "https://example.com/path",
		"https://user@example.com", "https://example.com?q",
		"https://example.com#f", "https://*.example.com", "https://example.com:",
		"https://example.com:http",
	} {
		_, ok := NormalizeOrigin(in)
		assert.False(t, ok, in)
	}
}

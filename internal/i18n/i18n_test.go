package i18n

import (
	"maps"
	"regexp"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

func placeholders(msg string) []string {
	var names []string
	for _, m := range placeholderPattern.FindAllStringSubmatch(msg, -1) {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	slices.Sort(names)
	return names
}

func TestCatalogParity(t *testing.T) {
	ref := Get(Reference)
	require.NotNil(t, ref)
	refKeys := slices.Sorted(maps.Keys(ref.messages))
	for _, lang := range Supported() {
		c := Get(lang)
		keys := slices.Sorted(maps.Keys(c.messages))
		assert.Equal(t, refKeys, keys, "%s: keys differ from %s", lang, Reference)
		for key, msg := range ref.messages {
			if other, ok := c.messages[key]; ok {
				want := placeholders(msg)
				got := placeholders(other)
				assert.Equal(t, want, got, "%s: placeholders of %s", lang, key)
			}
		}
	}
}

func TestCatalogLegalSlugs(t *testing.T) {
	assert := assert.New(t)
	slug := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	for _, lang := range Supported() {
		c := Get(lang)
		assert.NotEmpty(c.Name, lang)
		assert.Regexp(slug, c.Imprint, lang)
		assert.Regexp(slug, c.Privacy, lang)
		assert.NotEqual(c.Imprint, c.Privacy, "%s: legal slugs must differ", lang)
	}
}

func TestCatalogErrorCodes(t *testing.T) {
	for _, lang := range Supported() {
		for _, code := range apierr.Codes() {
			assert.Contains(t, Get(lang).messages, "error."+code, lang)
		}
	}
}

func TestLoad(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, _, err := load(fstest.MapFS{"de.json": {Data: []byte(`{}`)}})
	require.ErrorContains(err, "reference catalog")

	_, _, err = load(fstest.MapFS{
		"en.json": {Data: []byte(`{}`)},
		"EN.json": {Data: []byte(`{}`)},
	})
	require.ErrorContains(err, "invalid language code")

	_, _, err = load(fstest.MapFS{"en.json": {Data: []byte(`{"a": 1}`)}})
	require.ErrorContains(err, "strings or objects")

	c, langs, err := load(fstest.MapFS{
		"en.json":    {Data: []byte(`{"language": {"name": "English"}, "a": {"b": "x"}}`)},
		"pt-br.json": {Data: []byte(`{"language": {"name": "Português"}}`)},
	})
	require.NoError(err)
	assert.Equal([]string{"en", "pt-br"}, langs)
	assert.Equal("Português", c["pt-br"].Name)
	assert.Equal("x", c["en"].messages["a.b"])
}

func TestT(t *testing.T) {
	messages := map[string]string{"greet": "Hello { name }, {name} and {other}!"}
	c := &Catalog{messages: messages}
	params := map[string]string{"name": "Ann"}
	assert.Equal(t, "Hello Ann, Ann and {other}!", c.T("greet", params))
	assert.Equal(t, "missing.key", c.T("missing.key", nil))
}

func TestValidCode(t *testing.T) {
	for _, s := range []string{"de", "en", "pt-br"} {
		assert.True(t, ValidCode(s), s)
	}
	for _, s := range []string{"", "d", "deu", "DE", "pt-BR", "pt_br", "pt-bra"} {
		assert.False(t, ValidCode(s), s)
	}
}

func TestNegotiate(t *testing.T) {
	enabled := []string{"de", "en", "pt-br"}
	tests := []struct {
		name, cookie, header, want string
	}{
		{"primary without preferences", "", "", "en"},
		{"cookie wins", "de", "en", "de"},
		{"cookie for a disabled language is ignored", "fr", "", "en"},
		{"exact match", "", "de", "de"},
		{"q order", "", "de;q=0.5, pt-BR;q=0.8", "pt-br"},
		{"primary subtag", "", "fr, de-AT", "de"},
		{"exact match before primary subtag of a later tag", "", "en-US, de", "en"},
		{"q=0 excluded", "", "de;q=0", "en"},
		{"wildcard ignored", "", "*", "en"},
		{"malformed q skipped", "", "de;q=x, pt-br", "pt-br"},
		{"subtag does not widen the enabled language", "", "pt", "en"},
		{"equal q keeps order", "", "pt-br;q=0.5, de;q=0.5", "pt-br"},
	}
	for _, tt := range tests {
		got := Negotiate(enabled, "en", tt.cookie, tt.header)
		assert.Equal(t, tt.want, got, tt.name)
	}
}

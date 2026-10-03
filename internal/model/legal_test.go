package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegalNormalize(t *testing.T) {
	s := DefaultSettings()
	s.Legal = Legal{
		Imprint: LegalPage{
			Mode: LegalURL,
			URL:  Text{"en": " https://acme.example/imprint "},
			Text: Text{"en": "dropped"},
		},
		Privacy: LegalPage{Text: Text{"en": "dropped"}},
	}
	s.Landing = Text{
		"en": " Welcome ",
		"de": " ",
	}
	s.Normalize()
	require.NoError(t, s.Validate())
	want := Legal{
		Imprint: LegalPage{
			Mode: LegalURL,
			URL:  Text{"en": "https://acme.example/imprint"},
		},
		Privacy: LegalPage{Mode: LegalNone},
	}
	assert.Equal(t, want, s.Legal, "fields the mode does not use are dropped")
	assert.Equal(t, Text{"en": "Welcome"}, s.Landing)
}

func TestLegalValidate(t *testing.T) {
	tests := []struct {
		name string
		page LegalPage
		want map[string]string
	}{
		{"none", LegalPage{Mode: LegalNone}, nil},
		{"inherit", LegalPage{Mode: LegalInherit}, map[string]string{"legal.imprint.mode": "invalid_value"}},
		{"url", LegalPage{Mode: LegalURL, URL: Text{"en": "https://acme.example/imprint#top", "de": "http://acme.example"}}, nil},
		{"url required in primary", LegalPage{Mode: LegalURL, URL: Text{"de": "https://acme.example"}}, map[string]string{"legal.imprint.url.en": "required"}},
		{"relative url", LegalPage{Mode: LegalURL, URL: Text{"en": "/imprint"}}, map[string]string{"legal.imprint.url.en": "invalid_link"}},
		{"url without host", LegalPage{Mode: LegalURL, URL: Text{"en": "https:imprint"}}, map[string]string{"legal.imprint.url.en": "invalid_link"}},
		{"other scheme", LegalPage{Mode: LegalURL, URL: Text{"en": "javascript:alert(1)"}}, map[string]string{"legal.imprint.url.en": "invalid_link"}},
		{"url too long", LegalPage{Mode: LegalURL, URL: Text{"en": "https://acme.example/" + strings.Repeat("a", 1980)}}, map[string]string{"legal.imprint.url.en": "too_long"}},
		{"text", LegalPage{Mode: LegalText, Text: Text{"en": "**Acme**"}}, nil},
		{"text required in primary", LegalPage{Mode: LegalText}, map[string]string{"legal.imprint.text.en": "required"}},
		{"text too long", LegalPage{Mode: LegalText, Text: Text{"en": strings.Repeat("a", 50001)}}, map[string]string{"legal.imprint.text.en": "too_long"}},
	}
	for _, tt := range tests {
		s := DefaultSettings()
		s.Legal.Imprint = tt.page
		assert.Equal(t, tt.want, fieldCodes(t, s.Validate()), tt.name)
	}

	s := DefaultSettings()
	s.Landing = Text{"en": strings.Repeat("a", 50001)}
	want := map[string]string{"landing.en": "too_long"}
	assert.Equal(t, want, fieldCodes(t, s.Validate()))
}

func TestLegalEffective(t *testing.T) {
	instance := LegalPage{
		Mode: LegalText,
		Text: Text{"en": "Instance"},
	}
	own := LegalPage{
		Mode: LegalURL,
		URL:  Text{"en": "https://acme.example"},
	}
	assert.Equal(t, instance, LegalPage{Mode: LegalInherit}.Effective(instance))
	assert.Equal(t, own, own.Effective(instance))
	assert.Equal(
		t,
		LegalPage{Mode: LegalNone},
		LegalPage{Mode: LegalNone}.Effective(instance),
		"none does not inherit",
	)
}

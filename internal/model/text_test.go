package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

func TestTextResolve(t *testing.T) {
	langs := Languages{
		Enabled: []string{"de", "en", "fr"},
		Primary: "en",
	}
	tests := []struct {
		name string
		text Text
		lang string
		want string
	}{
		{"requested language", Text{"de": "Hallo", "en": "Hello"}, "de", "Hallo"},
		{"falls back to primary", Text{"en": "Hello", "fr": "Bonjour"}, "de", "Hello"},
		{"falls back to first enabled with value", Text{"fr": "Bonjour"}, "de", "Bonjour"},
		{"skips disabled languages", Text{"it": "Ciao"}, "de", ""},
		{"empty", nil, "de", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.text.Resolve(tt.lang, langs), tt.name)
	}
}

func TestLanguagesEffective(t *testing.T) {
	want := Languages{
		Enabled: []string{"de", "en"},
		Primary: "en",
	}
	langs := Languages{
		Enabled: []string{"de", "en"},
		Primary: "en",
	}
	assert.Equal(t, want, langs.Effective())

	want = Languages{
		Enabled: []string{"de", "en"},
		Primary: "de",
	}
	langs = Languages{
		Enabled: []string{"xx", "de", "en"},
		Primary: "xx",
	}
	assert.Equal(
		t,
		want,
		langs.Effective(),
		"a removed primary language is replaced by the first remaining one",
	)

	want = Languages{
		Enabled: []string{"en"},
		Primary: "en",
	}
	langs = Languages{
		Enabled: []string{"xx"},
		Primary: "xx",
	}
	assert.Equal(
		t,
		want,
		langs.Effective(),
		"without any remaining language, the reference language is used",
	)
}

func TestSettingsValidate(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	require.NoError(DefaultSettings().Validate())

	fields := func(s Settings) []apierr.Field {
		err := s.Validate()
		require.Error(err)
		e, ok := errors.AsType[*apierr.Error](err)
		require.True(ok)
		return e.Fields
	}

	want := []apierr.Field{
		{
			Path: "languages.enabled",
			Code: "required",
		},
		{
			Path: "languages.primary",
			Code: "required",
		},
		{
			Path: "defaultTheme",
			Code: "invalid_value",
		},
	}
	assert.Equal(want, fields(Settings{}))

	want = []apierr.Field{
		{
			Path: "languages.enabled[1]",
			Code: "unsupported_language",
		},
		{
			Path: "languages.enabled[2]",
			Code: "duplicate",
		},
		{
			Path: "languages.primary",
			Code: "primary_not_enabled",
		},
	}
	s := Settings{
		Languages: Languages{
			Enabled: []string{"en", "xx", "en"},
			Primary: "de",
		},
		DefaultTheme: "dark",
	}
	assert.Equal(want, fields(s))
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	assert.Equal(t, "en", s.Languages.Primary)
	assert.Equal(t, []string{"en", "de"}, s.Languages.Enabled)
	assert.Equal(t, "system", s.DefaultTheme)
}

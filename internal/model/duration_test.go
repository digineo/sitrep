package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	valid := map[string]time.Duration{
		"90s":      90 * time.Second,
		"1h30m":    90 * time.Minute,
		"7d":       7 * 24 * time.Hour,
		"1d2h3m4s": 24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second,
		"0s":       0,
		"24h":      24 * time.Hour,
	}
	for in, want := range valid {
		got, err := ParseDuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{
		"", "30", "1.5h", "1w", "30ms", "1m1h", "1h1h", "-1s", " 1s", "1s ", "h",
		"1H", "99999999999999999999d",
	} {
		_, err := ParseDuration(in)
		assert.Error(t, err, in)
	}
}

func TestFormatDuration(t *testing.T) {
	for in, want := range map[time.Duration]string{
		90 * time.Second:                "1m30s",
		24 * time.Hour:                  "1d",
		0:                               "0s",
		26*time.Hour + 5*time.Second:    "1d2h5s",
		1500 * time.Millisecond:         "1s",
		90*24*time.Hour + 1*time.Minute: "90d1m",
	} {
		assert.Equal(t, want, FormatDuration(in))
	}
}

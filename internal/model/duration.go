package model

import (
	"errors"
	"math"
	"strconv"
	"time"
)

var errDuration = errors.New(
	"invalid duration: " +
		"use whole numbers with the units d, h, m and s in that order, e.g. 1h30m",
)

var durationUnits = []struct {
	suffix byte
	size   time.Duration
}{
	{'d', 24 * time.Hour},
	{'h', time.Hour},
	{'m', time.Minute},
	{'s', time.Second},
}

// ParseDuration parses durations like "90s", "1h30m" or "7d". Each unit may
// appear once, in descending order.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errDuration
	}

	var total time.Duration
	next := 0
	for s != "" {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i == len(s) {
			return 0, errDuration
		}

		n, err := strconv.ParseInt(s[:i], 10, 64)
		if err != nil {
			return 0, errDuration
		}

		u := next
		for u < len(durationUnits) && durationUnits[u].suffix != s[i] {
			u++
		}
		if u == len(durationUnits) {
			return 0, errDuration
		}

		size := durationUnits[u].size
		if n > int64(math.MaxInt64-total)/int64(size) {
			return 0, errDuration
		}

		total += time.Duration(n) * size
		next = u + 1
		s = s[i+1:]
	}
	return total, nil
}

// FormatDuration returns the canonical form of d, e.g. "1m30s" or "1d".
// Fractions of a second are dropped.
func FormatDuration(d time.Duration) string {
	var b []byte
	for _, u := range durationUnits {
		if n := d / u.size; n > 0 {
			b = strconv.AppendInt(b, int64(n), 10)
			b = append(b, u.suffix)
			d -= n * u.size
		}
	}
	if b == nil {
		return "0s"
	}
	return string(b)
}

package i18n

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// Negotiate picks one of the enabled languages: the cookie's language, then
// the best match of the Accept-Language header, then the primary language.
func Negotiate(enabled []string, primary, cookie, acceptLanguage string) string {
	if slices.Contains(enabled, cookie) {
		return cookie
	}

	for _, tag := range acceptedTags(acceptLanguage) {
		if slices.Contains(enabled, tag) {
			return tag
		}

		prefix, _, ok := strings.Cut(tag, "-")
		if ok && slices.Contains(enabled, prefix) {
			return prefix
		}
	}
	return primary
}

// acceptedTags returns the lowercase tags of an Accept-Language header in
// descending q order, without wildcards and tags with q=0.
func acceptedTags(header string) []string {
	type weighted struct {
		tag string
		q   float64
	}
	var tags []weighted
	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(part, ";")
		tag = strings.ToLower(strings.TrimSpace(tag))
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			var err error
			if q, err = strconv.ParseFloat(v, 64); err != nil {
				continue
			}
		}
		if tag != "" && tag != "*" && q > 0 {
			tags = append(tags, weighted{tag, q})
		}
	}

	slices.SortStableFunc(tags, func(a, b weighted) int {
		return cmp.Compare(b.q, a.q)
	})

	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.tag
	}
	return out
}

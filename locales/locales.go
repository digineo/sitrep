// Package locales embeds the translation catalogs, one JSON file per
// supported language.
package locales

import "embed"

// FS holds the catalog files, named <language code>.json.
//
//go:embed *.json
var FS embed.FS

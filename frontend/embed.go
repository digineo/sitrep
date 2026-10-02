// Package frontend embeds the production build of the single-page
// applications, which Vite writes to dist/app, and the product logo.
package frontend

import "embed"

// Dist holds dist/app, or only a placeholder if the frontend is not built.
//
//go:embed all:dist
var Dist embed.FS

// Logo is the product logo, an SVG image.
//
//go:embed src/shared/assets/logo.svg
var Logo []byte

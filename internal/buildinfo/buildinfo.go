// Package buildinfo describes the build. The linker sets its variables,
// see the build target in GNUmakefile; they are empty otherwise.
package buildinfo

var (
	Version string // the release tag, if the commit has one
	Commit  string // the commit hash
	Date    string // the build time in RFC 3339
)

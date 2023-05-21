package terramate

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

// ErrVersion indicates failure when checking Terramate version.

// Version of terramate.
func Version() string {
	return strings.TrimSpace(version)
}

package terrastack

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

// Version of terrastack.
func Version() string {
	return strings.TrimSpace(version)
}

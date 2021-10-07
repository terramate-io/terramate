package terrastack

import (
	_ "embed"
	"strings"
)

var (

	//go:embed VERSION
	version string
)

// Version of terrastack.
func Version() string { return strings.TrimSpace(version) }

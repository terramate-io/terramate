package terrastack

import (
	_ "embed"
	"strings"
)

var (
	afs FS

	//go:embed VERSION
	version string
)

func init() {
	afs = osFS{}
}

// Version of terrastack.
func Version() string { return strings.TrimSpace(version) }

// Setup configures the terrastack abstractions for mocking/testing purposes.
// The afs is a custom implementation of FS.
func Setup(fs FS) {
	afs = fs
}

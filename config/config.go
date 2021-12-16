package config

import (
	"os"
	"path/filepath"
)

const (
	// Filename is the name of the terramate configuration file.
	Filename = "terramate.tm.hcl"

	// DefaultInitConstraint is the default constraint used in stack initialization.
	DefaultInitConstraint = "~>"
)

// Exists tells if path has a terramate config file.
func Exists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !st.IsDir() {
		return false
	}

	fname := filepath.Join(path, Filename)
	info, err := os.Stat(fname)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package plugin

import (
	"os"
	"path/filepath"
)

const terramateUserDirName = ".terramate.d"

// DefaultUserTerramateDir returns the default user terramate directory.
func DefaultUserTerramateDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, terramateUserDirName), nil
}

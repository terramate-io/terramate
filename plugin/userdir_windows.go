// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package plugin

import (
	"os"
	"path/filepath"
)

const terramateUserDirName = ".terramate.d"

// DefaultUserTerramateDir returns the default user terramate directory.
func DefaultUserTerramateDir() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", os.ErrNotExist
	}
	return filepath.Join(appdata, terramateUserDirName), nil
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package cliconfig

import (
	"os"
	"path/filepath"
)

// Filename is the name of the CLI configuration file.
const Filename = "terramate.rc"

// DirEnv is the environment variable used to define the config location.
const DirEnv = "APPDATA"

func configAbsPath() (string, bool) {
	appdata := os.Getenv(DirEnv)
	if appdata == "" {
		return "", false
	}
	return filepath.Join(appdata, Filename), true
}

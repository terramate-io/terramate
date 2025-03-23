// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package cli

import (
	"os"
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

func userTerramateDir() (string, error) {
	appdata := os.Getenv(cliconfig.DirEnv)
	if appdata == "" {
		return "", errors.E("APPDATA not set")
	}
	return filepath.Join(appdata, terramateUserConfigDir), nil
}

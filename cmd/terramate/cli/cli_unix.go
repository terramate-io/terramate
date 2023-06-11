// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package cli

import (
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
)

func userTerramateDir() (string, error) {
	homeDir, err := userHomeDir()
	if err != nil {
		return "", errors.E(err, "failed to discover the location of the local %s directory", terramateUserConfigDir)
	}
	return filepath.Join(homeDir, terramateUserConfigDir), nil
}

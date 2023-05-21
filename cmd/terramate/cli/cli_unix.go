// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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

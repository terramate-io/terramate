// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package cliconfig

import (
	"os/user"
	"path/filepath"
)

// Filename is the name of the CLI configuration file.
const Filename = ".terramaterc"

// DirEnv is the environment variable used to define the config location.
const DirEnv = "HOME"

func configAbsPath() (string, bool) {
	usr, err := user.Current()
	if err != nil {
		return "", false
	}
	return filepath.Join(usr.HomeDir, Filename), true
}

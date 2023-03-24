//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package cliconfig

import (
	"os/user"
	"path/filepath"
)

// Filename is the name of the CLI configuration file.
const Filename = ".terramaterc"

func configAbsPath() (string, bool) {
	usr, err := user.Current()
	if err != nil {
		return "", false
	}
	return filepath.Join(usr.HomeDir, Filename), true
}

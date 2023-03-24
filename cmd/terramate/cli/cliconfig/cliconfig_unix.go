package cliconfig

import (
	"os/user"
	"path/filepath"
)

const Filename = ".terramaterc"

func configAbsPath() (string, bool) {
	usr, err := user.Current()
	if err != nil {
		return "", false
	}
	return filepath.Join(usr.HomeDir, Filename), true
}

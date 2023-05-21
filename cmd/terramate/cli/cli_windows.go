// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package cli

import (
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/errors"
	"os"
	"path/filepath"
)

func userTerramateDir() (string, error) {
	appdata := os.Getenv(cliconfig.DirEnv)
	if appdata == "" {
		return "", errors.E("APPDATA not set")
	}
	return filepath.Join(appdata, terramateUserConfigDir), nil
}

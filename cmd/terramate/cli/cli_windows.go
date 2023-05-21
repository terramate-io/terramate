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

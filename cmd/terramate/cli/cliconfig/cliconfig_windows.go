//go:build windows

package cliconfig

// Filename is the name of the CLI configuration file.
const Filename = "terramate.rc"

func configAbsPath() (string, bool) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", false
	}
	return filepath.Join(appdata, Filename), true
}

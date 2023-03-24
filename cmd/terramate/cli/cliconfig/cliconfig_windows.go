package cliconfig

const Filename = "terramate.rc"

func configAbsPath() (string, bool) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", false
	}
	return filepath.Join(appdata, Filename), true
}

package terrastack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigFilename is the name of the terrastack configuration file.
const ConfigFilename = "terrastack"

// Init initialize a stack. It's an error to initialize an already initialized
// stack unless they are of same versions. In case the stack is initialized with
// other terrastack version, the force flag can be used to explicitly initialize
// it anyway.
func Init(dir string, force bool) error {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("init requires an existing directory")
		}

		return fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	if !st.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	stackfile := filepath.Join(dir, ConfigFilename)
	isInitialized := false

	st, err = os.Stat(stackfile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat failed on %q: %w", stackfile, err)
		}
	} else {
		isInitialized = true
	}

	if isInitialized && !st.Mode().IsRegular() {
		return fmt.Errorf("the path %q is not a regular file", stackfile)
	}

	if isInitialized && !force {
		version, err := parseVersion(stackfile)
		if err != nil {
			return fmt.Errorf("stack already initialized: error fetching "+
				"version: %w", err)
		}

		if version != Version() {
			return fmt.Errorf("stack already initialized with version %q "+
				"but terrastack version is %q", version, Version())
		}

		err = os.Remove(string(stackfile))
		if err != nil {
			return fmt.Errorf("while removing %q: %w", stackfile, err)
		}
	}

	err = os.WriteFile(stackfile, []byte(Version()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write %q: %w", stackfile, err)
	}

	return nil
}

func parseVersion(stackfile string) (string, error) {
	data, err := os.ReadFile(stackfile)
	if err != nil {
		return "", fmt.Errorf("reading stack file: %w", err)
	}

	if len(strings.Split(string(data), ".")) != 3 {
		return "", fmt.Errorf("wrong version number: %q", string(data))
	}

	return string(data), nil
}

package terrastack

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Init initialize a stack. It's an error to initialize an already initialized
// stack unless they are of same versions. In case the stack is initialized with
// other terrastack version, the force flag can be used to explicitly initialize
// it anyway.
func Init(dir Dirname, force bool) error {
	exists, err := dir.Check()
	if err != nil {
		return fmt.Errorf("while checking \"%s\": %w", dir, err)
	}

	if !exists {
		return fmt.Errorf("init requires an existing directory")
	}

	stackfile := Filename(filepath.Join(string(dir), "terrastack"))

	exists, err = stackfile.Check()
	if err != nil {
		return fmt.Errorf("while checking file \"%s\": %w", stackfile, err)
	}

	if exists && !force {
		version, err := parseVersion(stackfile)
		if err != nil {
			return fmt.Errorf("stack already initialized: error fetching "+
				"version: %w", err)
		}

		if version != Version() {
			return fmt.Errorf("stack initialized with other version %s "+
				"(terrastack version is %s", version, Version())
		}

		err = afs.Remove(string(stackfile))
		if err != nil {
			return fmt.Errorf("while removing \"%s\": %w", stackfile, err)
		}
	}

	err = WriteFile(afs, stackfile, []byte(Version()))
	if err != nil {
		return fmt.Errorf("failed to write \"%s\": %w", stackfile, err)
	}

	return nil
}

func parseVersion(stackfile Filename) (string, error) {
	data, err := fs.ReadFile(afs, string(stackfile))
	if err != nil {
		return "", fmt.Errorf("reading stack file: %w", err)
	}

	if len(strings.Split(string(data), ".")) != 3 {
		return "", fmt.Errorf("wrong version number: %s", string(data))
	}

	return string(data), nil
}

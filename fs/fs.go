// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"os"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// ListTerramateFiles returns a list of terramate related files from the
// directory dir.
func ListTerramateFiles(dir string) ([]string, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate files")
	}

	files := []string{}

	for _, dirEntry := range dirEntries {
		if strings.HasPrefix(dirEntry.Name(), ".") {
			continue
		}

		if dirEntry.IsDir() {
			continue
		}

		filename := dirEntry.Name()
		if isTerramateFile(filename) {
			files = append(files, filename)
		}
	}

	return files, nil
}

// ListTerramateDirs lists Terramate dirs, which are any dirs
// except ones starting with ".".
func ListTerramateDirs(dir string) ([]string, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate dirs")
	}

	dirs := []string{}

	for _, dirEntry := range dirEntries {

		if !dirEntry.IsDir() {
			continue
		}

		if strings.HasPrefix(dirEntry.Name(), ".") {
			continue
		}

		dirs = append(dirs, dirEntry.Name())
	}

	return dirs, nil
}

func isTerramateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl")
}

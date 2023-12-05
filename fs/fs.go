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
func ListTerramateFiles(dir string) (filenames []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, errors.E(err, "opening directory %s for reading file entries", dir)
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate files")
	}

	files := []string{}

	for _, entry := range dirEntries {
		fname := entry.Name()
		if entry.IsDir() || !isTerramateFile(fname) {
			continue
		}
		files = append(files, fname)
	}

	return files, nil
}

// ListTerramateDirs lists Terramate dirs, which are any dirs
// except ones starting with ".".
func ListTerramateDirs(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, errors.E(err, "opening directory %s for reading file entries", dir)
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate dirs")
	}

	dirs := []string{}

	for _, dirEntry := range dirEntries {
		fname := dirEntry.Name()
		if fname[0] == '.' || !dirEntry.IsDir() {
			continue
		}
		dirs = append(dirs, fname)
	}
	return dirs, nil
}

func isTerramateFile(filename string) bool {
	if len(filename) <= 3 || filename[0] == '.' {
		return false
	}
	switch filename[len(filename)-1] {
	default:
		return false
	case 'l':
		return strings.HasSuffix(filename, ".tm.hcl")
	case 'm':
		return strings.HasSuffix(filename, ".tm")
	}
}

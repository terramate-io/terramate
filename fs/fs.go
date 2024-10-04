// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"os"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// ListTerramateFiles returns the entries of directory separated (terramate files, others  and
// directories)
func ListTerramateFiles(dir string) (tmFiles []string, otherFiles []string, dirs []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, nil, nil, errors.E(err, "opening directory %s for reading file entries", dir)
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return nil, nil, nil, errors.E(err, "reading dir to list Terramate files")
	}

	for _, entry := range dirEntries {
		fname := entry.Name()
		if entry.IsDir() && fname[0] != '.' {
			dirs = append(dirs, fname)
		} else if isTerramateFile(fname) {
			tmFiles = append(tmFiles, fname)
		} else {
			otherFiles = append(otherFiles, fname)
		}
	}
	return tmFiles, otherFiles, dirs, nil
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

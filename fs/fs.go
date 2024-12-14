// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"os"
	"sort"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// ListResult contains the result of listing a directory.
type ListResult struct {
	TmFiles    []string
	TmGenFiles []string
	OtherFiles []string
	Dirs       []string
}

// ListTerramateFiles returns the entries of directory separated (terramate files, others  and
// directories)
func ListTerramateFiles(dir string) (ListResult, error) {
	f, err := os.Open(dir)
	if err != nil {
		return ListResult{}, errors.E(err, "opening directory %s for reading file entries", dir)
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return ListResult{}, errors.E(err, "reading dir to list Terramate files")
	}

	const tmgenExt = ".tmgen"

	res := ListResult{}
	for _, entry := range dirEntries {
		fname := entry.Name()
		if fname[0] == '.' {
			res.OtherFiles = append(res.OtherFiles, fname)
			continue
		}
		if entry.IsDir() {
			res.Dirs = append(res.Dirs, fname)
		} else if isTerramateFile(fname) {
			res.TmFiles = append(res.TmFiles, fname)
		} else if strings.HasSuffix(fname, tmgenExt) && len(fname) > len(tmgenExt) {
			res.TmGenFiles = append(res.TmGenFiles, fname)
		} else {
			res.OtherFiles = append(res.OtherFiles, fname)
		}
	}
	sort.Strings(res.Dirs)
	sort.Strings(res.TmFiles)
	sort.Strings(res.TmGenFiles)
	sort.Strings(res.OtherFiles)
	return res, nil
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

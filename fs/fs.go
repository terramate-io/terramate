// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"os"
	"sort"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

const tmgenExt = ".tmgen"

// ListResult contains the result of listing a directory.
type ListResult struct {
	TmFiles    []string
	TmGenFiles []string
	OtherFiles []string
	Dirs       []string
	Skipped    []string
}

// AddFile adds a file to the ListResult. It classifies the file accordingly.
func (r *ListResult) AddFile(name string) {
	switch {
	case name[0] == '.':
		r.Skipped = append(r.Skipped, name)
	case isTerramateFile(name):
		r.TmFiles = append(r.TmFiles, name)
	case strings.HasSuffix(name, tmgenExt) && len(name) > len(tmgenExt):
		r.TmGenFiles = append(r.TmGenFiles, name)
	default:
		r.OtherFiles = append(r.OtherFiles, name)
	}
}

// AddDir adds a directory to the ListResult. It classifies the directory accordingly.
func (r *ListResult) AddDir(name string) {
	if name[0] == '.' {
		r.Skipped = append(r.Skipped, name)
	} else {
		r.Dirs = append(r.Dirs, name)
	}
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

	res := ListResult{}
	for _, entry := range dirEntries {
		fname := entry.Name()
		if entry.IsDir() {
			res.AddDir(fname)
		} else {
			res.AddFile(fname)
		}
	}
	sort.Strings(res.Dirs)
	sort.Strings(res.TmFiles)
	sort.Strings(res.TmGenFiles)
	sort.Strings(res.OtherFiles)
	sort.Strings(res.Skipped)
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

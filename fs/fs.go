// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// ListTerramateFiles returns a list of terramate related files from the
// directory dir. Optionally you can provide a list of inspect funcs which,
// if provided, gets executed for each name in the directory.
func ListTerramateFiles(dir string, inspect ...func(name string)) ([]string, error) {
	names, err := readdir(dir)
	if err != nil {
		return nil, errors.E(err, "reading directory file names")
	}
	return FilterTerramateFiles(dir, names, inspect...)
}

// ListTerramateDirs lists Terramate dirs, which are any dirs
// except ones starting with ".". Optionally you can provide a list of
// inspectFuncs which, if provided, gets executed for each name in the directory.
func ListTerramateDirs(dir string, inspect ...func(name string)) ([]string, error) {
	names, err := readdir(dir)
	if err != nil {
		return nil, errors.E(err, "reading directory file names")
	}
	return FilterDirs(dir, names, inspect...)
}

// FilterTerramateFiles filter the names list and returns only the Terramate
// file names. Optionally you can provide a list of inspect funcs which,
// if provided, gets executed for each name in the directory.
func FilterTerramateFiles(
	basedir string, names []string, inspect ...func(name string),
) ([]string, error) {
	var tmnames []string
	for _, name := range names {
		for _, cb := range inspect {
			cb(name)
		}
		if Skip(name) {
			continue
		}
		dir := filepath.Join(basedir, name)
		st, err := os.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			continue
		}
		if st.Mode().IsRegular() && isTerramateFile(name) {
			tmnames = append(tmnames, name)
		}
	}
	return tmnames, nil
}

// FilterDirs filter directories from names which are of interest of Terramate.
func FilterDirs(
	basedir string, names []string, inspect ...func(name string),
) ([]string, error) {
	var tmdirs []string
	for _, name := range names {
		for _, cb := range inspect {
			cb(name)
		}
		if Skip(name) {
			continue
		}
		dir := filepath.Join(basedir, name)
		st, err := os.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			continue
		}
		tmdirs = append(tmdirs, name)
	}
	return tmdirs, nil
}

func readdir(dir string) (names []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, errors.E(err, "failed to open cfg directory")
	}

	defer func() {
		errs := errors.L(err, f.Close())
		err = errs.AsError()
	}()

	names, err = f.Readdirnames(0)
	if err == nil {
		sort.Strings(names)
	}
	return names, err
}

func isTerramateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl")
}

// Skip returns true if the given file/dir name should be ignored by Terramate.
func Skip(name string) bool {
	// assumes filename length > 0
	return name[0] == '.'
}

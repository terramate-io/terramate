// Copyright 2021 Mineiros GmbH
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

package terramate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/hcl"
)

type Stack struct {
	Name string
	Dir  string

	config *hcl.Terramate
}

func LoadStack(fname string) (Stack, error) {
	tm, err := hcl.ParseFile(fname)
	if err != nil {
		return Stack{}, err
	}
	name := filepath.Base(fname)
	stackdir := strings.TrimSuffix(fname, fmt.Sprintf("/%s", name))
	return Stack{
		Name:   name,
		Dir:    stackdir,
		config: tm,
	}, nil
}

// IsStack tells if path is a stack and if so then it returns the stackfile path.
func IsStack(info fs.FileInfo, path string) (bool, string) {
	if !info.IsDir() {
		return false, ""
	}

	fname := filepath.Join(path, ConfigFilename)
	info, err := os.Stat(fname)
	if err != nil {
		return false, ""
	}

	if info.Mode().IsRegular() {
		return true, fname
	}
	return false, ""
}

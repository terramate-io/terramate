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
	Dir string

	block *hcl.Stack
}

// LoadStack loads the stack from dir directory.
func LoadStack(dir string) (Stack, error) {
	fname := filepath.Join(dir, ConfigFilename)
	cfg, err := hcl.ParseFile(fname)
	if err != nil {
		return Stack{}, err
	}

	if cfg.Stack == nil {
		return Stack{}, fmt.Errorf("no stack found in %q", dir)
	}

	name := filepath.Base(fname)
	stackdir := strings.TrimSuffix(fname, fmt.Sprintf("/%s", name))
	return Stack{
		Dir:   stackdir,
		block: cfg.Stack,
	}, nil
}

// LoadStacks loads all the stacks in the dirs directories. If dirs are relative
// paths, then basedir is used as base.
func LoadStacks(basedir string, dirs ...string) ([]Stack, error) {
	stacks := []Stack{}

	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(basedir, d)
		}
		stack, err := LoadStack(d)
		if err != nil {
			return nil, err
		}

		stacks = append(stacks, stack)
	}
	return stacks, nil
}

// IsStack tells if path is a stack and if so then it returns the stackfile path.
func IsStack(info fs.FileInfo, path string) bool {
	if !info.IsDir() {
		return false
	}

	fname := filepath.Join(path, ConfigFilename)
	info, err := os.Stat(fname)
	if err != nil {
		return false
	}

	if info.Mode().IsRegular() {
		return true
	}
	return false
}

func (s Stack) Name() string {
	if s.block.Name != "" {
		return s.block.Name
	}
	return filepath.Base(s.Dir)
}

func (s Stack) After() []string { return s.block.After }

func (s Stack) String() string {
	return s.Name()
}

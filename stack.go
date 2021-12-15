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
	"path/filepath"

	"github.com/mineiros-io/terramate/hcl"
)

type Stack struct {
	name string
	Dir  string

	block *hcl.Stack
}

// LoadStack loads a stack from dir directory.
func LoadStack(dir string) (Stack, error) {
	fname := filepath.Join(dir, ConfigFilename)
	cfg, err := hcl.ParseFile(fname)
	if err != nil {
		return Stack{}, err
	}

	if cfg.Stack == nil {
		return Stack{}, fmt.Errorf("no stack found in %q", dir)
	}

	ok, err := isLeafDirectory(dir)
	if err != nil {
		return Stack{}, err
	}

	if !ok {
		return Stack{}, fmt.Errorf("stack %q is not a leaf directory", dir)
	}

	return stackFromBlock(dir, cfg.Stack), nil
}

// TryLoadStack tries to load a stack from directory. It returns found as true
// only in the case that path contains a stack and it was correctly parsed.
func TryLoadStack(dir string) (stack Stack, found bool, err error) {
	if ok := HasConfig(dir); !ok {
		return Stack{}, false, err
	}
	fname := filepath.Join(dir, ConfigFilename)
	cfg, err := hcl.ParseFile(fname)
	if err != nil {
		return Stack{}, false, err
	}

	if cfg.Stack == nil {
		return Stack{}, false, nil
	}

	ok, err := isLeafDirectory(dir)
	if err != nil {
		return Stack{}, false, err
	}

	if !ok {
		return Stack{}, false, fmt.Errorf("stack %q is not a leaf directory", dir)
	}

	return stackFromBlock(dir, cfg.Stack), true, nil
}

func stackFromBlock(dir string, block *hcl.Stack) Stack {
	var name string
	if block.Name != "" {
		name = block.Name
	} else {
		name = filepath.Base(dir)
	}

	return Stack{
		name:  name,
		Dir:   dir,
		block: block,
	}
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

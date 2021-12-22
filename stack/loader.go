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

package stack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
)

// Loader is a stack loader.
type Loader map[string]S

// NewLoader creates a new stack loader.
func NewLoader() Loader {
	return make(Loader)
}

// Load loads a stack from dir directory. If the stack was previously loaded, it
// returns the cached one.
func (l Loader) Load(root, dir string) (S, error) {
	stackpath := project.RelPath(root, dir)
	if s, ok := l[stackpath]; ok {
		return s, nil
	}

	fname := filepath.Join(dir, config.Filename)
	cfg, err := hcl.ParseFile(fname)
	if err != nil {
		return S{}, err
	}

	if cfg.Stack == nil {
		return S{}, fmt.Errorf("no stack found in %q", dir)
	}

	ok, err := l.IsLeafStack(root, dir)
	if err != nil {
		return S{}, err
	}

	if !ok {
		return S{}, fmt.Errorf("stack %q is not a leaf directory", dir)
	}

	l.set(stackpath, cfg.Stack)
	return l[stackpath], nil
}

// LoadChanged is like Load but sets the stack as changed if loaded
// successfully.
func (l Loader) LoadChanged(projectdir, dir string) (S, error) {
	s, err := l.Load(projectdir, dir)
	if err != nil {
		return S{}, err
	}

	s.changed = true
	return s, nil
}

// TryLoad tries to load a stack from directory. It returns found as true
// only in the case that path contains a stack and it was correctly parsed.
// It caches the stack for later use.
func (l Loader) TryLoad(root, dir string) (stack S, found bool, err error) {
	if !strings.HasPrefix(dir, root) {
		return S{}, false, fmt.Errorf("directory %q is not inside project root %q",
			dir, root)
	}
	stackpath := project.RelPath(root, dir)
	if s, ok := l[stackpath]; ok {
		return s, true, nil
	}

	if ok := config.Exists(dir); !ok {
		return S{}, false, err
	}
	fname := filepath.Join(dir, config.Filename)
	cfg, err := hcl.ParseFile(fname)

	if errors.Is(err, hcl.ErrNoTerramateBlock) {
		// Config blocks may have only globals, no terramate
		return S{}, false, nil
	}

	if err != nil {
		return S{}, false, err
	}

	if cfg.Stack == nil {
		return S{}, false, nil
	}

	ok, err := l.IsLeafStack(root, dir)
	if err != nil {
		return S{}, false, err
	}

	if !ok {
		return S{}, false, fmt.Errorf("stack %q is not a leaf stack", dir)
	}

	l.set(stackpath, cfg.Stack)
	return l[stackpath], true, nil
}

// TryLoadChanged is like TryLoad but sets the stack as changed if loaded
// successfully.
func (l Loader) TryLoadChanged(root, dir string) (stack S, found bool, err error) {
	s, ok, err := l.TryLoad(root, dir)
	if ok {
		s.changed = true
	}
	return s, ok, err
}

func (l Loader) set(path string, block *hcl.Stack) {
	var name string
	if block.Name != "" {
		name = block.Name
	} else {
		name = filepath.Base(path)
	}

	l[path] = S{
		name:  name,
		Dir:   path,
		block: block,
	}
}

func (l Loader) Set(dir string, s S) {
	l[dir] = s
}

// LoadAll loads all the stacks in the dirs directories. If dirs are relative
// paths, then basedir is used as base.
func (l Loader) LoadAll(root string, basedir string, dirs ...string) ([]S, error) {
	stacks := []S{}

	absbase := filepath.Join(root, basedir)

	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(absbase, d)
		}
		stack, err := l.Load(root, d)
		if err != nil {
			return nil, err
		}

		stacks = append(stacks, stack)
	}
	return stacks, nil
}

func (l Loader) IsLeafStack(root, dir string) (bool, error) {
	isValid := true
	err := filepath.Walk(
		dir,
		func(path string, info fs.FileInfo, err error) error {
			if !isValid {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			if path == dir {
				return nil
			}
			if info.IsDir() {
				if strings.HasSuffix(path, "/.git") {
					return filepath.SkipDir
				}

				_, found, err := l.TryLoad(root, path)
				if err != nil {
					return err
				}

				isValid = !found
				return nil
			}
			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return isValid, nil
}

func (l Loader) lookupParentStack(root, dir string) (stack S, found bool, err error) {
	if root == dir {
		return S{}, false, nil
	}
	d := filepath.Dir(dir)
	for {
		stack, ok, err := l.TryLoad(root, d)
		if err != nil {
			return S{}, false, fmt.Errorf("looking for parent stacks: %w", err)
		}

		if ok {
			return stack, true, nil
		}

		if d == root || d == "/" {
			break
		}

		gitpath := filepath.Join(d, ".git")
		if _, err := os.Stat(gitpath); err == nil {
			// if reached root of git project, abort scanning
			break
		}

		d = filepath.Dir(d)
	}

	return S{}, false, nil
}

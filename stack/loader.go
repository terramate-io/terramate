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
type Loader struct {
	root   string
	stacks map[string]S
}

// NewLoader creates a new stack loader for project's root directory.
func NewLoader(root string) Loader {
	return Loader{
		root:   root,
		stacks: make(map[string]S),
	}
}

// Load loads a stack from dir directory. If the stack was previously loaded, it
// returns the cached one.
func (l Loader) Load(dir string) (S, error) {
	stackpath := project.RelPath(l.root, dir)
	if s, ok := l.stacks[stackpath]; ok {
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

	ok, err := l.IsLeafStack(dir)
	if err != nil {
		return S{}, err
	}

	if !ok {
		return S{}, fmt.Errorf("stack %q is not a leaf directory", dir)
	}

	l.set(stackpath, cfg.Stack)
	return l.stacks[stackpath], nil
}

// TryLoad tries to load a stack from directory. It returns found as true
// only in the case that path contains a stack and it was correctly parsed.
// It caches the stack for later use.
func (l Loader) TryLoad(dir string) (stack S, found bool, err error) {
	if !strings.HasPrefix(dir, l.root) {
		return S{}, false, fmt.Errorf("directory %q is not inside project root %q",
			dir, l.root)
	}
	stackpath := project.RelPath(l.root, dir)
	if s, ok := l.stacks[stackpath]; ok {
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

	ok, err := l.IsLeafStack(dir)
	if err != nil {
		return S{}, false, err
	}

	if !ok {
		return S{}, false, fmt.Errorf("stack %q is not a leaf stack", dir)
	}

	l.set(stackpath, cfg.Stack)
	return l.stacks[stackpath], true, nil
}

// TryLoadChanged is like TryLoad but sets the stack as changed if loaded
// successfully.
func (l Loader) TryLoadChanged(root, dir string) (stack S, found bool, err error) {
	s, ok, err := l.TryLoad(dir)
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

	l.stacks[path] = S{
		name:  name,
		Dir:   path,
		block: block,
	}
}

// Set stacks in the loader's cache. The dir directory must be relative to
// project's root.
func (l Loader) Set(dir string, s S) {
	l.stacks[dir] = s
}

// MustGet must return a stack from the loader's cache. It's a programming error
// to call this method if the stack wasn't loaded before.
func (l Loader) MustGet(dir string) S {
	return l.stacks[dir]
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
		stack, err := l.Load(d)
		if err != nil {
			return nil, err
		}

		stacks = append(stacks, stack)
	}
	return stacks, nil
}

func (l Loader) IsLeafStack(dir string) (bool, error) {
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

				_, found, err := l.TryLoad(path)
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

func (l Loader) lookupParentStack(dir string) (stack S, found bool, err error) {
	if l.root == dir {
		return S{}, false, nil
	}
	d := filepath.Dir(dir)
	for {
		stack, ok, err := l.TryLoad(d)
		if err != nil {
			return S{}, false, fmt.Errorf("looking for parent stacks: %w", err)
		}

		if ok {
			return stack, true, nil
		}

		if d == l.root || d == "/" {
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

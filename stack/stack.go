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
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

type (
	// S represents a stack
	S struct {
		// abspath is the file system absolute path of the stack.
		abspath string

		// prjAbsPath is the absolute path of the stack relative to project's root.
		prjAbsPath string

		// name of the stack.
		name string

		// description of the stack.
		description string

		// after is a list of stack paths that must run before this stack.
		after []string

		// before is a list of stack paths that must run after this stack.
		before []string

		// wants is the list of stacks that must be selected whenever this stack
		// is selected.
		wants []string

		// changed tells if this is a changed stack.
		changed bool
	}

	// Metadata has all metadata loaded per stack
	Metadata struct {
		Name        string
		Path        string
		Description string
	}
)

// New creates a new stack from configuration cfg.
func New(root string, cfg hcl.Config) S {
	name := cfg.Stack.Name
	if name == "" {
		name = filepath.Base(cfg.AbsDir())
	}

	return S{
		name:        name,
		description: cfg.Stack.Description,
		after:       cfg.Stack.After,
		before:      cfg.Stack.Before,
		wants:       cfg.Stack.Wants,
		abspath:     cfg.AbsDir(),
		prjAbsPath:  project.PrjAbsPath(root, cfg.AbsDir()),
	}
}

// Name of the stack.
func (s S) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.PrjAbsPath()
}

// Description of stack.
func (s S) Description() string { return s.description }

// After specifies the list of stacks that must run before this stack.
func (s S) After() []string { return s.after }

// Before specifies the list of stacks that must run after this stack.
func (s S) Before() []string { return s.before }

func (s S) Wants() []string { return s.wants }

// IsChanged tells if the stack is marked as changed.
func (s S) IsChanged() bool { return s.changed }

// SetChanged sets the changed flag of the stack.
func (s *S) SetChanged(b bool) { s.changed = b }

// String representation of the stack.
func (s S) String() string { return s.Name() }

// PrjAbsPath returns the project's absolute path of stack.
func (s S) PrjAbsPath() string { return s.prjAbsPath }

// AbsPath returns the file system absolute path of stack.
func (s S) AbsPath() string { return s.abspath }

// Meta returns the stack metadata.
func (s S) Meta() Metadata {
	return Metadata{
		Name:        s.Name(),
		Path:        s.PrjAbsPath(),
		Description: s.Description(),
	}
}

func (m Metadata) ToCtyMap() map[string]cty.Value {
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name),
		"path":        cty.StringVal(m.Path),
		"description": cty.StringVal(m.Description),
	}
}

func IsLeaf(root, dir string) (bool, error) {
	l := NewLoader(root)
	return l.IsLeafStack(dir)
}

func LookupParent(root, dir string) (S, bool, error) {
	l := NewLoader(root)
	return l.lookupParentStack(dir)
}

// Load a single stack from dir.
func Load(root, dir string) (S, error) {
	l := NewLoader(root)
	return l.Load(dir)
}

// TryLoad tries to load a single stack from dir. It sets found as true in case
// the stack was successfully loaded.
func TryLoad(root, absdir string) (stack S, found bool, err error) {
	logger := log.With().
		Str("action", "TryLoad()").
		Str("dir", absdir).
		Logger()

	if !strings.HasPrefix(absdir, root) {
		return S{}, false, fmt.Errorf("directory %q is not inside project root %q",
			absdir, root)
	}

	if ok := config.Exists(absdir); !ok {
		return S{}, false, err
	}

	fname := filepath.Join(absdir, config.Filename)

	logger.Debug().
		Str("configFile", fname).
		Msg("Parse config file.")

	cfg, err := hcl.ParseFile(fname)
	if err != nil {
		return S{}, false, err
	}

	if cfg.Stack == nil {
		return S{}, false, nil
	}

	ok, err := IsLeaf(root, absdir)
	if err != nil {
		return S{}, false, err
	}

	if !ok {
		return S{}, false, fmt.Errorf("stack %q is not a leaf stack", absdir)
	}

	logger.Debug().Msg("Create a new stack")

	return New(root, cfg), true, nil
}

func Sort(stacks []S) {
	sort.Sort(stackSlice(stacks))
}

// stackSlice implements the Sort interface.
type stackSlice []S

func (l stackSlice) Len() int           { return len(l) }
func (l stackSlice) Less(i, j int) bool { return l[i].PrjAbsPath() < l[j].PrjAbsPath() }
func (l stackSlice) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

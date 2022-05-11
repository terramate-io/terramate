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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
)

type (
	// S represents a stack
	S struct {
		// hostpath is the file system absolute path of the stack.
		hostpath string

		// path is the absolute path of the stack relative to project's root.
		path string

		// name of the stack.
		name string

		// desc is the description of the stack.
		desc string

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
	Metadata interface {
		Name() string
		Path() string
		Desc() string
	}
)

// New creates a new stack from configuration cfg.
func New(root string, cfg hcl.Config) S {
	name := cfg.Stack.Name
	if name == "" {
		name = filepath.Base(cfg.AbsDir())
	}

	return S{
		name:     name,
		desc:     cfg.Stack.Description,
		after:    cfg.Stack.After,
		before:   cfg.Stack.Before,
		wants:    cfg.Stack.Wants,
		hostpath: cfg.AbsDir(),
		path:     project.PrjAbsPath(root, cfg.AbsDir()),
	}
}

// Name of the stack.
func (s S) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.Path()
}

// Desc is the description of the stack.
func (s S) Desc() string { return s.desc }

// After specifies the list of stacks that must run before this stack.
func (s S) After() []string { return s.after }

// Before specifies the list of stacks that must run after this stack.
func (s S) Before() []string { return s.before }

// Wants specifies the list of wanted stacks.
func (s S) Wants() []string { return s.wants }

// IsChanged tells if the stack is marked as changed.
func (s S) IsChanged() bool { return s.changed }

// SetChanged sets the changed flag of the stack.
func (s *S) SetChanged(b bool) { s.changed = b }

// String representation of the stack.
func (s S) String() string { return s.Path() }

// Path returns the project's absolute path of stack.
func (s S) Path() string { return s.path }

// HostPath returns the file system absolute path of stack.
func (s S) HostPath() string { return s.hostpath }

// IsLeaf returns true if dir is a leaf stack.
func IsLeaf(root, dir string) (bool, error) {
	l := NewLoader(root)
	return l.IsLeafStack(dir)
}

// LookupParent checks parent stack of given dir.
// Returns false, nil if the given dir has no parent stack.
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
		return S{}, false, errors.E(fmt.Sprintf("directory %q is not inside project root %q",
			absdir, root))
	}

	logger.Debug().Msg("Parsing configuration.")
	cfg, err := hcl.ParseDir(absdir)
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
		return S{}, false, errors.E(fmt.Sprintf("stack %q is not a leaf stack", absdir))
	}

	logger.Debug().Msg("Create a new stack")
	return New(root, cfg), true, nil
}

// Sort sorts the given stacks.
func Sort(stacks []S) {
	sort.Sort(stackSlice(stacks))
}

// Reverse reverses the given stacks slice.
func Reverse(stacks []S) {
	i, j := 0, len(stacks)-1
	for i < j {
		stacks[i], stacks[j] = stacks[j], stacks[i]
		i++
		j--
	}
}

// stackSlice implements the Sort interface.
type stackSlice []S

func (l stackSlice) Len() int           { return len(l) }
func (l stackSlice) Less(i, j int) bool { return l[i].Path() < l[j].Path() }
func (l stackSlice) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

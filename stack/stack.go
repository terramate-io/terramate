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
	"path/filepath"

	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

type (
	// S represents a stack
	S struct {
		// Dir is the stack dir path relative to the project root
		Dir string

		// name of the stack.
		name string

		// description of the stack.
		description string

		// after is a list of stack paths that must run before this stack.
		after []string

		// before is a list of stack paths that must run after this stack.
		before []string

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
		Dir:         project.RelPath(root, cfg.AbsDir()),
		description: cfg.Stack.Description,
		after:       cfg.Stack.After,
		before:      cfg.Stack.Before,
	}
}

// Name of the stack.
func (s S) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.Dir
}

// Description of stack.
func (s S) Description() string { return s.description }

// After specifies the list of stacks that must run before this stack.
func (s S) After() []string { return s.after }

// Before specifies the list of stacks that must run after this stack.
func (s S) Before() []string { return s.before }

// IsChanged tells if the stack is marked as changed.
func (s S) IsChanged() bool { return s.changed }

// SetChanged sets the changed flag of the stack.
func (s *S) SetChanged(b bool) { s.changed = b }

// String representation of the stack.
func (s S) String() string { return s.Name() }

// Meta returns the stack metadata.
func (s S) Meta() Metadata {
	return Metadata{
		Name:        s.Name(),
		Path:        s.Dir,
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
func TryLoad(root, dir string) (stack S, found bool, err error) {
	l := NewLoader(root)
	return l.TryLoad(dir)
}

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
	"github.com/zclconf/go-cty/cty"
)

type (
	// S represents a stack
	S struct {
		// Dir is the stack dir path relative to the project root
		Dir string

		name    string
		changed bool
		block   *hcl.Stack
	}

	// Metadata has all metadata loaded per stack
	Metadata struct {
		Name        string
		Path        string
		Description string
	}
)

func (s S) Name() string {
	if s.block.Name != "" {
		return s.block.Name
	}
	return filepath.Base(s.Dir)
}

func (s S) Description() string {
	return s.block.Description
}

func (s S) After() []string  { return s.block.After }
func (s S) Before() []string { return s.block.Before }

func (s S) IsChanged() bool    { return s.changed }
func (s *S) SetChanged(b bool) { s.changed = b }

func (s S) String() string {
	return s.Name()
}

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

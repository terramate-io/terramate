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
	"strings"
)

// StackMetadata has all metadata loaded per stack
type StackMetadata struct {
	Name string
	Path string
}

// Metadata has all metadata loader per project
type Metadata struct {
	// Stacks is a lexycographicaly sorted (by stack path) list of stack metadata
	Stacks []StackMetadata

	basedir string
}

// LoadMetadata loads the project metadata given the project basedir.
func LoadMetadata(basedir string) (Metadata, error) {
	stackEntries, err := ListStacks(basedir)
	if err != nil {
		return Metadata{}, err
	}

	stacksMetadata := make([]StackMetadata, len(stackEntries))
	for i, stackEntry := range stackEntries {
		stacksMetadata[i] = StackMetadata{
			Name: stackEntry.Stack.Name(),
			Path: strings.TrimPrefix(stackEntry.Stack.Dir, basedir),
		}
	}

	return Metadata{
		Stacks:  stacksMetadata,
		basedir: basedir,
	}, nil
}

// StackMetadata gets the metadata of a specific stack given its absolute path.
func (m Metadata) StackMetadata(abspath string) (StackMetadata, bool) {
	path := strings.TrimPrefix(abspath, m.basedir)
	for _, stackMetadata := range m.Stacks {
		if stackMetadata.Path == path {
			return stackMetadata, true
		}
	}
	return StackMetadata{}, false
}

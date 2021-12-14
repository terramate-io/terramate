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
	"path/filepath"
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
}

// LoadMetadata loads the project metadata given the project basedir.
func LoadMetadata(basedir string) (Metadata, error) {
	stacks, err := ListStacks(basedir)
	if err != nil {
		return Metadata{}, err
	}

	stacksMetadata := make([]StackMetadata, len(stacks))
	for i, stack := range stacks {
		stacksMetadata[i] = StackMetadata{
			Name: filepath.Base(stack.Dir),
			Path: strings.TrimPrefix(stack.Dir, basedir),
		}
	}

	return Metadata{
		Stacks: stacksMetadata,
	}, nil
}

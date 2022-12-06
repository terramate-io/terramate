// Copyright 2022 Mineiros GmbH
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

// Package trigger provides functionality that help manipulate stacks triggers.
package trigger

import "github.com/mineiros-io/terramate/project"

// Raw represents the raw contents of a trigger.
type Raw struct {
	filename string
	body     string
}

// Triggers represents all triggers of a given project.
// It also provides a way to create new triggers.
type Triggers struct {
	triggers map[project.Path][]Raw
}

// Load will load all stack triggers for the project rooted at rootdir.
func Load(rootdir string) *Triggers {
	return &Triggers{
		triggers: map[project.Path][]Raw{},
	}
}

// Create creates a trigger for a stack with the given path and the given reason.
func (t *Triggers) Create(path project.Path, reason string) error {
	return nil
}

// Has returns true if there is a trigger for a stack with the given path.
func (t *Triggers) Has(path project.Path) (bool, error) {
	return false, nil
}

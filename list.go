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
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/stack"
)

// ListStacks walks the config tree looking for terramate stacks.
// It returns a lexicographic sorted list of stack directories.
func ListStacks(cfg *config.Tree) ([]Entry, error) {
	stacks, err := stack.LoadAll(cfg)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, len(stacks))
	for i, s := range stacks {
		entries[i] = Entry{Stack: s}
	}
	return entries, nil
}

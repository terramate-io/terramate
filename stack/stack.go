// Copyright 2023 Mineiros GmbH
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

import "github.com/mineiros-io/terramate/config"

// List loads from the config all terramate stacks.
// It returns a lexicographic sorted list of stack directories.
func List(cfg *config.Tree) ([]Entry, error) {
	stacks, err := config.LoadAllStacks(cfg)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, len(stacks))
	for i, elem := range stacks {
		entries[i] = Entry{Stack: elem.Stack}
	}
	return entries, nil
}

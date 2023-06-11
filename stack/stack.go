// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import "github.com/terramate-io/terramate/config"

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

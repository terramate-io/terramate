// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run_test

import (
	"path"
	"slices"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestStableSortOrder(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:s1",
		"s:s1/s2",
		"s:s1/s2/s3",
		"s:s1/s2/s3/s4",
		"s:s1/s2/s3/s4/s5",
	})

	root, err := config.LoadRoot(s.RootDir())
	assert.NoError(t, err)

	type item struct {
		index int
		stack *config.Stack
	}
	var items []item

	// We define 5 stacks, due to parent relationship there is an implicit ordering between them. s1 -> s5.
	stackOrder := []string{"s1/s2/s3/s4/s5", "s1/s2/s3/s4", "s1/s2/s3", "s1/s2", "s1"}

	// For each stack, we add 100 numbered items, starting at s5 -> s1.
	for _, sname := range stackOrder {
		s, err := config.LoadStack(root, project.NewPath(path.Join("/", sname)))
		assert.NoError(t, err)

		for i := 0; i < 100; i++ {
			items = append(items, item{index: i + 123, stack: s})
		}
	}

	// We do a topological sorting of the items.
	getStack := func(item item) *config.Stack { return item.stack }
	_, err = run.Sort(root, items, getStack)
	assert.NoError(t, err)

	slices.Reverse(stackOrder)

	// We expect the items in order of s1 -> s5 after sorting, so the stack order is reversed,
	// but ordering within each stack is preserved (0 -> 100).
	for i, sname := range stackOrder {
		for j := 0; j < 100; j++ {
			got := items[i*100+j]
			assert.EqualStrings(t, sname, got.stack.RelPath(), "matrix: %v %v", i, j)
			assert.EqualInts(t, j+123, got.index)
		}
	}
}

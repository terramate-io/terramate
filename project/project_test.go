// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package project_test

import (
	"testing"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
)

func TestPrjAbsPathOnRoot(t *testing.T) {
	path := project.PrjAbsPath("/", "/file.hcl")
	test.AssertEqualPaths(t, path, project.NewPath("/file.hcl"))
}

func TestFriendlyFmtDir(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		wd       string
		dir      string
		expected string
		ok       bool
	}{
		// Current directory
		{
			name:     "current directory",
			root:     "/project",
			wd:       "/project/stacks/a",
			dir:      "/stacks/a",
			expected: ".",
			ok:       true,
		},
		// Descendant paths (existing behavior)
		{
			name:     "child directory",
			root:     "/project",
			wd:       "/project/stacks",
			dir:      "/stacks/a",
			expected: "a",
			ok:       true,
		},
		{
			name:     "grandchild directory",
			root:     "/project",
			wd:       "/project/stacks",
			dir:      "/stacks/a/b",
			expected: "a/b",
			ok:       true,
		},
		// Sibling paths (new behavior)
		{
			name:     "sibling directory",
			root:     "/project",
			wd:       "/project/stacks/a",
			dir:      "/stacks/b",
			expected: "../b",
			ok:       true,
		},
		{
			name:     "sibling with subdirectory",
			root:     "/project",
			wd:       "/project/stacks/a",
			dir:      "/stacks/b/c",
			expected: "../b/c",
			ok:       true,
		},
		// Cousin paths
		{
			name:     "cousin directory",
			root:     "/project",
			wd:       "/project/stacks/a/x",
			dir:      "/stacks/b/y",
			expected: "../../b/y",
			ok:       true,
		},
		// Parent paths
		{
			name:     "parent directory",
			root:     "/project",
			wd:       "/project/stacks/a",
			dir:      "/stacks",
			expected: "..",
			ok:       true,
		},
		{
			name:     "grandparent directory",
			root:     "/project",
			wd:       "/project/stacks/a/b",
			dir:      "/stacks",
			expected: "../..",
			ok:       true,
		},
		// Root directory
		{
			name:     "root directory",
			root:     "/project",
			wd:       "/project/stacks/a",
			dir:      "/",
			expected: "../..",
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := project.FriendlyFmtDir(tt.root, tt.wd, tt.dir)
			if ok != tt.ok {
				t.Errorf("FriendlyFmtDir() ok = %v, want %v", ok, tt.ok)
			}
			if result != tt.expected {
				t.Errorf("FriendlyFmtDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

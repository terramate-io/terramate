// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestStackClone(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name    string
		layout  []string
		src     string
		dest    string
		wantErr error
	}

	testcases := []testcase{
		{
			name:   "clone simple stack",
			layout: []string{"s:/stack"},
			src:    "/stack",
			dest:   "/stack-cloned",
		},
		{
			name: "clone stack with subdirs",
			layout: []string{
				"s:/stack",
				"f:/stack/somestackfile:test",
				"f:/stack/subdir/file:test",
				"f:/stack/subdir2/file2:test",
				"f:/stack/subdir2/subdir3/file3:test",
			},
			src:  "/stack",
			dest: "/stack-cloned",
		},
		{
			name: "clone stack to dest with subdirs",
			layout: []string{
				"s:/stack",
				"f:/stack/subdir/file:test",
				"f:/stack/subdir2/file2:test",
				"f:/stack/subdir2/subdir3/file3:test",
			},
			src:  "/stack",
			dest: "/dir/subdir/cloned-stack",
		},
		{
			name:    "src dir must be stack",
			layout:  []string{"d:/not-stack"},
			src:     "/not-stack",
			dest:    "/new-stack",
			wantErr: errors.E(stack.ErrInvalidStackDir),
		},
		{
			name:    "src dir must exist",
			src:     "/non-existent-stack",
			dest:    "/new-stack",
			wantErr: errors.E(stack.ErrInvalidStackDir),
		},
		{
			name: "dest dir must not exist",
			layout: []string{
				"s:/stack",
				"d:/cloned-stack",
			},
			src:     "/stack",
			dest:    "/cloned-stack",
			wantErr: errors.E(stack.ErrCloneDestDirExists),
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)

			srcdir := filepath.Join(s.RootDir(), tc.src)
			destdir := filepath.Join(s.RootDir(), tc.dest)
			err := stack.Clone(s.Config(), destdir, srcdir)
			assert.IsError(t, err, tc.wantErr)

			if tc.wantErr != nil {
				return
			}

			test.AssertTreeEquals(t, srcdir, destdir)
		})
	}
}

func TestStackCloneSrcDirMustBeInsideRootdir(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	srcdir := test.TempDir(t)
	destdir := filepath.Join(s.RootDir(), "new-stack")
	err := stack.Clone(s.Config(), destdir, srcdir)
	assert.IsError(t, err, errors.E(stack.ErrInvalidStackDir))
}

func TestStackCloneTargetDirMustBeInsideRootdir(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	srcdir := filepath.Join(s.RootDir(), "src-stack")
	destdir := test.TempDir(t)
	err := stack.Clone(s.Config(), destdir, srcdir)
	assert.IsError(t, err, errors.E(stack.ErrInvalidStackDir))
}

func TestStackCloneIgnoresDotDirsAndFiles(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:stack",
		"f:stack/.dotfile",
		"f:stack/.dotdir/file",
	})
	srcdir := filepath.Join(s.RootDir(), "stack")
	destdir := filepath.Join(s.RootDir(), "cloned-stack")
	err := stack.Clone(s.Config(), destdir, srcdir)
	assert.NoError(t, err)

	entries := test.ReadDir(t, destdir)
	assert.EqualInts(t, 1, len(entries), "expected only stack config file to be copied, got: %v", entriesNames(entries))
	assert.EqualStrings(t, stack.DefaultFilename, entries[0].Name())
}

func TestStackCloneIfStackHasIDClonedStackHasNewUUID(t *testing.T) {
	t.Parallel()
	const (
		stackID          = "stack-id"
		stackName        = "stack name"
		stackDesc        = "stack description"
		stackCfgFilename = "stack.tm.hcl"
		stackCfgTemplate = `
// Commenting generate_hcl 1
generate_hcl "test.hcl" {
  content {
    // Commenting literal
    a = "literal"
    // Commenting expression
    b = tm_try(global.expression, null)
  }
}

// Some comments
/*
  Commenting is fun
*/

stack {
  // Commenting stack ID
  id = %q // comment after ID expression
  // Commenting stack name
  name = %q // More comments !!
  // Commenting stack description
  description = %q
}

generate_hcl "test2.hcl" {
  content {
    b = tm_try(global.expression, null)
    a = "literal"
  }
}
`
	)
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{"d:stack"})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateFile(stackCfgFilename, fmt.Sprintf(stackCfgTemplate,
		stackID, stackName, stackDesc))

	srcdir := filepath.Join(s.RootDir(), "stack")
	destdir := filepath.Join(s.RootDir(), "cloned-stack")

	err := stack.Clone(s.Config(), destdir, srcdir)
	assert.NoError(t, err)

	cfg := test.ParseTerramateConfig(t, destdir)
	if cfg.Stack == nil {
		t.Fatalf("cloned stack has no stack block: %v", cfg)
	}

	if cfg.Stack.ID == "" {
		t.Fatalf("cloned stack has no ID: %v", cfg.Stack)
	}

	if cfg.Stack.ID == stackID {
		t.Fatalf("want cloned stack to have different ID, got %s == %s", cfg.Stack.ID, stackID)
	}

	assert.EqualStrings(t, stackName, cfg.Stack.Name)
	assert.EqualStrings(t, stackDesc, cfg.Stack.Description)

	want := fmt.Sprintf(stackCfgTemplate, cfg.Stack.ID, stackName, stackDesc)

	clonedStackEntry := s.DirEntry("cloned-stack")
	got := string(clonedStackEntry.ReadFile(stackCfgFilename))

	assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)
}

func TestStackClonesTags(t *testing.T) {
	t.Parallel()
	const (
		stackName        = "stack name"
		stackDesc        = "stack description"
		stackCfgFilename = "stack.tm.hcl"
		stackCfgTemplate = `
stack {
  // Commenting stack name
  name = %q // More comments !!
  // Commenting stack description
  description = %q

  tags = ["a", "b", "c"]
}
`
	)
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{"d:stack"})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateFile(stackCfgFilename, fmt.Sprintf(stackCfgTemplate,
		stackName, stackDesc))

	srcdir := filepath.Join(s.RootDir(), "stack")
	destdir := filepath.Join(s.RootDir(), "cloned-stack")

	err := stack.Clone(s.Config(), destdir, srcdir)
	assert.NoError(t, err)

	cfg := test.ParseTerramateConfig(t, destdir)
	if cfg.Stack == nil {
		t.Fatalf("cloned stack has no stack block: %v", cfg)
	}

	assert.EqualStrings(t, stackName, cfg.Stack.Name)
	assert.EqualStrings(t, stackDesc, cfg.Stack.Description)

	assert.EqualInts(t, len(cfg.Stack.Tags), 3)
	assert.EqualStrings(t, cfg.Stack.Tags[0], "a")
	assert.EqualStrings(t, cfg.Stack.Tags[1], "b")
	assert.EqualStrings(t, cfg.Stack.Tags[2], "c")

	want := fmt.Sprintf(stackCfgTemplate, stackName, stackDesc)

	clonedStackEntry := s.DirEntry("cloned-stack")
	got := string(clonedStackEntry.ReadFile(stackCfgFilename))

	assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)
}

func entriesNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, v := range entries {
		names[i] = v.Name()
	}
	return names
}

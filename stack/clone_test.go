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

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStackClone(t *testing.T) {
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
		{
			name: "src stack ID must be a string literal",
			layout: []string{
				`f:/stack/stack.tm.hcl:stack {
				  id = ("id")
				}`,
			},
			src:     "/stack",
			dest:    "/cloned-stack",
			wantErr: errors.E(stack.ErrCloneSrcStackIDNotSupported),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srcdir := filepath.Join(s.RootDir(), tc.src)
			destdir := filepath.Join(s.RootDir(), tc.dest)
			err := stack.Clone(s.RootDir(), destdir, srcdir)
			assert.IsError(t, err, tc.wantErr)

			if tc.wantErr != nil {
				return
			}

			test.AssertTreeEquals(t, srcdir, destdir)
		})
	}
}

func TestStackCloneSrcDirMustBeInsideRootdir(t *testing.T) {
	s := sandbox.New(t)
	srcdir := t.TempDir()
	destdir := filepath.Join(s.RootDir(), "new-stack")
	err := stack.Clone(s.RootDir(), destdir, srcdir)
	assert.IsError(t, err, errors.E(stack.ErrInvalidStackDir))
}

func TestStackCloneTargetDirMustBeInsideRootdir(t *testing.T) {
	s := sandbox.New(t)
	srcdir := filepath.Join(s.RootDir(), "src-stack")
	destdir := t.TempDir()
	err := stack.Clone(s.RootDir(), destdir, srcdir)
	assert.IsError(t, err, errors.E(stack.ErrInvalidStackDir))
}

func TestStackCloneIgnoresDotDirsAndFiles(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack",
		"f:stack/.dotfile",
		"f:stack/.dotdir/file",
	})
	srcdir := filepath.Join(s.RootDir(), "stack")
	destdir := filepath.Join(s.RootDir(), "cloned-stack")
	err := stack.Clone(s.RootDir(), destdir, srcdir)
	assert.NoError(t, err)

	entries := test.ReadDir(t, destdir)
	assert.EqualInts(t, 1, len(entries), "expected only stack config file to be copied, got: %v", entriesNames(entries))
	assert.EqualStrings(t, stack.DefaultFilename, entries[0].Name())
}

func TestStackCloneIfStackHasIDClonedStackHasNewUUID(t *testing.T) {
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

stack{
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
	s := sandbox.New(t)
	s.BuildTree([]string{"d:stack"})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateFile(stackCfgFilename, fmt.Sprintf(stackCfgTemplate,
		stackID, stackName, stackDesc))

	srcdir := filepath.Join(s.RootDir(), "stack")
	destdir := filepath.Join(s.RootDir(), "cloned-stack")

	err := stack.Clone(s.RootDir(), destdir, srcdir)
	assert.NoError(t, err)

	cfg := test.ParseTerramateConfig(t, destdir)
	if cfg.Stack == nil {
		t.Fatalf("cloned stack has no stack block: %v", cfg)
	}

	clonedStackID, ok := cfg.Stack.ID.Value()
	if !ok {
		t.Fatalf("cloned stack has no ID: %v", cfg.Stack)
	}

	if clonedStackID == stackID {
		t.Fatalf("want cloned stack to have different ID, got %s == %s", clonedStackID, stackID)
	}

	assert.EqualStrings(t, stackName, cfg.Stack.Name)
	assert.EqualStrings(t, stackDesc, cfg.Stack.Description)

	want := fmt.Sprintf(stackCfgTemplate, clonedStackID, stackName, stackDesc)

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

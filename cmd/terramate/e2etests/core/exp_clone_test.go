// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloneStack(t *testing.T) {
	t.Parallel()

	const (
		srcStack         = "stack"
		destStack        = "cloned-stack"
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
  }
}
`
	)
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{"d:stack"})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateFile(stackCfgFilename, fmt.Sprintf(stackCfgTemplate,
		stackID, stackName, stackDesc))

	tmcli := NewCLI(t, s.RootDir())
	res := tmcli.Run("experimental", "clone", srcStack, destStack)

	AssertRunResult(t, res, RunExpected{
		StdoutRegex: cloneSuccessMsg(1, srcStack, destStack),
	})

	destdir := filepath.Join(s.RootDir(), destStack)
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

	clonedStackEntry := s.DirEntry(destStack)
	got := string(clonedStackEntry.ReadFile(stackCfgFilename))

	assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)

	// Checking that code was also generated already
	genHCL := string(clonedStackEntry.ReadFile("test.hcl"))
	genHCL2 := string(clonedStackEntry.ReadFile("test2.hcl"))

	test.AssertGenCodeEquals(t, genHCL, `a = "literal"`)
	test.AssertGenCodeEquals(t, genHCL2, `b = null`)
}

func TestCloneStacksWithChildren(t *testing.T) {
	t.Parallel()

	type testcase struct {
		Name            string
		Layout          []string
		Srcdir          string
		Destdir         string
		SkipChildStacks bool
		WantLayout      []string
		WantError       string
		WantCloneCount  int
	}

	tests := []testcase{
		{
			Name: "clone stack dir with children",
			Layout: []string{
				"s:stack-1",
				"f:stack-1/.ignored",
				"f:stack-1/somefile.txt:foo",
				"s:stack-1/child-a",
				"s:stack-1/child-b",
				"s:stack-2",
				"s:stack-2/child-c",
			},
			Srcdir:  "stack-1",
			Destdir: "cloned-stack-1",
			WantLayout: []string{
				"s:cloned-stack-1",
				"s:cloned-stack-1/child-a",
				"s:cloned-stack-1/child-b",
				"f:cloned-stack-1/somefile.txt:foo",
				"!e:cloned-stack-1/.ignored",
			},
			WantCloneCount: 3,
		},
		{
			Name: "clone non-stack dir with children",
			Layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"s:stacks/stack-4",
			},
			Srcdir:  "stacks",
			Destdir: "cloned-stacks",
			WantLayout: []string{
				"s:cloned-stacks/stack-1",
				"s:cloned-stacks/stack-2",
				"s:cloned-stacks/stack-3",
				"s:cloned-stacks/stack-4",
			},
			WantCloneCount: 4,
		},
		{
			Name: "clone stack dir without children",
			Layout: []string{
				"s:stack-1",
				"s:stack-1/child-a",
				"s:stack-2",
				"f:stack-2/somefile.txt:foo",
				"f:stack-2/.ignored",
				"s:stack-2/child-b",
				"s:stack-2/child-c",
				"f:stack-2/mydir/others.txt:foo",
				"f:stack-2/mydir/stuff/.ignored2",
			},
			Srcdir:          "stack-2",
			Destdir:         "cloned-stack-2",
			SkipChildStacks: true,
			WantLayout: []string{
				"s:cloned-stack-2",
				"!e:cloned-stack-2/.ignored",
				"!e:cloned-stack-2/mydir/stuff/.ignored2",
				"f:cloned-stack-2/somefile.txt:foo",
				"f:cloned-stack-2/mydir/others.txt:foo",
				"f:cloned-stack-2/mydir/others.txt:foo",
			},
			WantCloneCount: 1,
		},
		{
			Name: "fail to clone non-stack dir if skipping children",
			Layout: []string{
				"f:stacks/somefile.txt:foo",
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"f:stacks/other/other.txt:bar",
			},
			Srcdir:          "stacks",
			Destdir:         "cloned-stacks",
			SkipChildStacks: true,
			WantError:       "no stacks to clone",
		},
		{
			Name: "destdir is in srcdir",
			Layout: []string{
				"s:stack-A",
				"s:stack-A/child-stack",
				"s:stack-A/child-stack/nested",
				"s:stack-A/stack-A-copied",
				"s:stack-A/stack-A-copied/child-stack",
				"s:stack-A/stack-A-copied/child-stack/nested",
			},
			WantLayout: []string{
				"s:stack-A/stack-A-copied/copied-again",
				"s:stack-A/stack-A-copied/copied-again/child-stack",
				"s:stack-A/stack-A-copied/copied-again/child-stack/nested",
				"s:stack-A/stack-A-copied/copied-again/stack-A-copied",
				"s:stack-A/stack-A-copied/copied-again/stack-A-copied/child-stack",
				"s:stack-A/stack-A-copied/copied-again/stack-A-copied/child-stack/nested",
			},
			Srcdir:         "stack-A",
			Destdir:        "stack-A/stack-A-copied/copied-again",
			WantCloneCount: 6,
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.Layout)

			tmcli := NewCLI(t, s.RootDir())
			args := []string{"experimental", "clone", tc.Srcdir, tc.Destdir}
			if tc.SkipChildStacks {
				args = append(args, "--skip-child-stacks")
			}
			res := tmcli.Run(args...)

			if tc.WantError == "" {
				AssertRunResult(t, res, RunExpected{
					StdoutRegex: cloneSuccessMsg(tc.WantCloneCount, tc.Srcdir, tc.Destdir),
				})
			} else {
				AssertRunResult(t, res, RunExpected{
					Status:      1,
					StderrRegex: tc.WantError,
				})
			}

			wantLayout := append(tc.Layout, tc.WantLayout...)
			s.AssertTree(wantLayout, sandbox.WithStrictStackValidation())
		})
	}
}

func TestCloneErrorCleanup(t *testing.T) {
	t.Parallel()

	const (
		dstdir = "cloned_dir"
	)

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"s:a",
		"s:a/b",
		"s:a/b/c",
	})

	dir := s.DirEntry("a/b/c").CreateDir("somedir")
	file := dir.CreateFile("file.txt", "")
	file.Chmod(0)
	defer file.Chmod(0755)

	tmcli := NewCLI(t, s.RootDir())
	res := tmcli.Run("experimental", "clone", "a", dstdir)

	assert.EqualInts(t, 1, res.Status, "clone exit status")

	test.DoesNotExist(t, s.RootDir(), dstdir)
}

func cloneSuccessMsg(c int, src, dst string) string {
	return fmt.Sprintf("Cloned %d stack\\(s\\) from %s to %s with success\n", c, src, dst)
}

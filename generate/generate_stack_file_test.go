// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestGenerateFile(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "empty generate_file content generates empty file",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateFile(
						Labels("empty"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"empty": stringer(""),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"empty": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"empty"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"empty"},
					},
				},
			},
		},
		{
			name: "generate_file with false condition generates nothing",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateFile(
						Labels("test"),
						Bool("condition", false),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "terramate.stacks.list",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateFile(
							Labels("stacks.txt"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"stacks.txt"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"stacks.txt"},
					},
				},
			},
		},
		{
			name: "generate_file with stack on root",
			layout: []string{
				"s:/",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateFile(
							Labels("root.txt"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"root.txt": stringer(`["/"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{"root.txt"},
					},
				},
			},
		},
		{
			name: "generate_file with stack on root and substacks",
			layout: []string{
				"s:/",
				"s:/stack-1",
				"s:/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateFile(
							Labels("root.txt"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"root.txt": stringer(`["/","/stack-1","/stack-2"]`),
					},
				},
				{
					dir: "/stack-1",
					files: map[string]fmt.Stringer{
						"root.txt": stringer(`["/","/stack-1","/stack-2"]`),
					},
				},
				{
					dir: "/stack-2",
					files: map[string]fmt.Stringer{
						"root.txt": stringer(`["/","/stack-1","/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{"root.txt"},
					},
					{
						Dir:     project.NewPath("/stack-1"),
						Created: []string{"root.txt"},
					},
					{
						Dir:     project.NewPath("/stack-2"),
						Created: []string{"root.txt"},
					},
				},
			},
		},
		{
			name: "generate files for all stacks from parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateFile(
							Labels("file1.txt"),
							Expr("content", "terramate.path"),
						),
						GenerateFile(
							Labels("file2.txt"),
							Expr("content", "terramate.name"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-1"),
						"file2.txt": stringer("stack-1"),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-2"),
						"file2.txt": stringer("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"file1.txt", "file2.txt"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			name: "generate files for single stack",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-2",
					add: Doc(
						GenerateFile(
							Labels("file1.txt"),
							Expr("content", "terramate.path"),
						),
						GenerateFile(
							Labels("file2.txt"),
							Expr("content", "terramate.name"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-2"),
						"file2.txt": stringer("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			name: "generate files using explicit context=stack attribute",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-2",
					add: Doc(
						GenerateFile(
							Labels("file1.txt"),
							Expr("content", "terramate.path"),
							Expr("context", "stack"),
						),
						GenerateFile(
							Labels("file2.txt"),
							Expr("content", "terramate.name"),
							Expr("context", "stack"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-2"),
						"file2.txt": stringer("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
	})
}

func TestGenerateFileRemoveFilesWhenConditionIsFalse(t *testing.T) {
	t.Parallel()

	const filename = "file.txt"

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")

	assertFileExist := func(file string) {
		t.Helper()

		path := filepath.Join(stackEntry.Path(), file)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("want file %q to exist, instead got: %v", path, err)
		}
	}
	assertFileDontExist := func(file string) {
		t.Helper()

		path := filepath.Join(stackEntry.Path(), file)
		_, err := os.Stat(path)

		if errors.Is(err, os.ErrNotExist) {
			return
		}

		t.Fatalf("want file %q to not exist, instead got: %v", path, err)
	}

	createConfig := func(filename string, condition bool) {
		stackEntry.CreateConfig(
			GenerateFile(
				Labels(filename),
				Bool("condition", condition),
				Str("content", "some content"),
			).String(),
		)
	}

	createConfig(filename, false)
	report := s.Generate()
	assertEqualReports(t, report, generate.Report{})
	assertFileDontExist(filename)

	createConfig(filename, true)
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     project.NewPath("/stack"),
				Created: []string{filename},
			},
		},
	})
	assertFileExist(filename)

	createConfig(filename, false)
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     project.NewPath("/stack"),
				Deleted: []string{filename},
			},
		},
	})
	assertFileDontExist(filename)
}

func TestGenerateFileTerramateRootMetadata(t *testing.T) {
	t.Parallel()

	// We need to know the sandbox abspath to test terramate.root properly
	const generatedFile = "file.hcl"

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	s.RootEntry().CreateConfig(
		Doc(
			GenerateFile(
				Labels(generatedFile),
				Expr("content", `"${terramate.root.path.fs.absolute}-${terramate.root.path.fs.basename}"`),
			),
		).String(),
	)

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     project.NewPath("/stack"),
				Created: []string{generatedFile},
			},
		},
	})

	want := s.RootDir() + "-" + filepath.Base(s.RootDir())
	got := stackEntry.ReadFile(generatedFile)

	assert.EqualStrings(t, want, got)
}

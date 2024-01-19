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

	s := sandbox.NoGit(t, true)
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

	s := sandbox.NoGit(t, true)
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

func TestGenerateFileStackFilters(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "no matching pattern",
			layout: []string{
				"s:staecks/stack-1",
				"s:staecks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/staecks",
					add: GenerateFile(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*"),
						),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "one matching pattern",
			layout: []string{
				"s:stacks/stack-1",
				"s:staecks/stack-2",
				"s:stack/stack-3",
				"s:stackss/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*"),
						),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test": stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "multiple matching patterns",
			layout: []string{
				"s:stacks/stack-1",
				"s:staecks/stack-2",
				"s:stack/stack-3",
				"s:staecks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*", "staecks/*"),
						),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test": stringer("content"),
					},
				},
				{
					dir: "/staecks/stack-2",
					files: map[string]fmt.Stringer{
						"test": stringer("content"),
					},
				},
				{
					dir: "/staecks/stack-4",
					files: map[string]fmt.Stringer{
						"test": stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/staecks/stack-2"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/staecks/stack-4"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "AND multiple attributes",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("not_generated"),
						StackFilter(
							ProjectPaths("stack-1"),
							RepositoryPaths("stack-2"),
						),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "OR multiple blocks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("generated"),
						StackFilter(
							ProjectPaths("stack-1"),
						),
						StackFilter(
							RepositoryPaths("stack-2"),
						),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack-1",
					files: map[string]fmt.Stringer{
						"generated": stringer("content"),
					},
				},
				{
					dir: "/stack-2",
					files: map[string]fmt.Stringer{
						"generated": stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack-1"),
						Created: []string{"generated"},
					},
					{
						Dir:     project.NewPath("/stack-2"),
						Created: []string{"generated"},
					},
				},
			},
		},
		{
			name: "glob patterns",
			layout: []string{
				"s:aws/stacks/dev",
				"s:aws/stacks/dev/substack",
				"s:aws/stacks/prod",
				"s:gcp/stacks/dev",
				"s:gcp/stacks/prod",
				"s:gcp/stacks/prod/substack",
				"s:dev",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("prod_match1"),
						StackFilter(
							ProjectPaths("prod"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("prod_match2"),
						StackFilter(
							ProjectPaths("**/prod"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("no_prod_match1"),
						StackFilter(
							ProjectPaths("*/prod"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("prod_substack_match1"),
						StackFilter(
							ProjectPaths("**/prod/*"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("prod_substack_match2"),
						StackFilter(
							ProjectPaths("**/prod/**"),
						),
						Str("content", "content"),
					),
				},

				{
					path: "/",
					add: GenerateFile(
						Labels("aws_prod_match1"),
						StackFilter(
							ProjectPaths("aws/**/prod"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("no_aws_substack_match1"),
						StackFilter(
							ProjectPaths("aws/*/substack"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("aws_substack_match1"),
						StackFilter(
							ProjectPaths("aws/**/substack"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("substack_match1"),
						StackFilter(
							ProjectPaths("substack"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("no_substack_match1"),
						StackFilter(
							ProjectPaths("/substack"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("root_dev_match1"),
						StackFilter(
							ProjectPaths("/dev"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("all_aws_match1"),
						StackFilter(
							ProjectPaths("aws/**"),
						),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("no_aws_match1"),
						StackFilter(
							ProjectPaths("aws/*"),
						),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/aws/stacks/dev",
					files: map[string]fmt.Stringer{
						"all_aws_match1": stringer("content"),
					},
				},
				{
					dir: "/aws/stacks/dev/substack",
					files: map[string]fmt.Stringer{
						"all_aws_match1":      stringer("content"),
						"aws_substack_match1": stringer("content"),
						"substack_match1":     stringer("content"),
					},
				},
				{
					dir: "/aws/stacks/prod",
					files: map[string]fmt.Stringer{
						"all_aws_match1":  stringer("content"),
						"aws_prod_match1": stringer("content"),
						"prod_match1":     stringer("content"),
						"prod_match2":     stringer("content"),
					},
				},
				{
					dir: "/dev",
					files: map[string]fmt.Stringer{
						"root_dev_match1": stringer("content"),
					},
				},
				{
					dir: "/gcp/stacks/prod",
					files: map[string]fmt.Stringer{
						"prod_match1": stringer("content"),
						"prod_match2": stringer("content"),
					},
				},
				{
					dir: "/gcp/stacks/prod/substack",
					files: map[string]fmt.Stringer{
						"prod_substack_match1": stringer("content"),
						"prod_substack_match2": stringer("content"),
						"substack_match1":      stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: project.NewPath("/aws/stacks/dev"),
						Created: []string{
							"all_aws_match1",
						},
					},
					{
						Dir: project.NewPath("/aws/stacks/dev/substack"),
						Created: []string{
							"all_aws_match1",
							"aws_substack_match1",
							"substack_match1",
						},
					},
					{
						Dir: project.NewPath("/aws/stacks/prod"),
						Created: []string{
							"all_aws_match1",
							"aws_prod_match1",
							"prod_match1",
							"prod_match2",
						},
					},
					{
						Dir: project.NewPath("/dev"),
						Created: []string{
							"root_dev_match1",
						},
					},
					{
						Dir: project.NewPath("/gcp/stacks/prod"),
						Created: []string{
							"prod_match1",
							"prod_match2",
						},
					},
					{
						Dir: project.NewPath("/gcp/stacks/prod/substack"),
						Created: []string{
							"prod_substack_match1",
							"prod_substack_match2",
							"substack_match1",
						},
					},
				},
			},
		},
	})
}

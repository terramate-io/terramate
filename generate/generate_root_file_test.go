// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"

	"github.com/terramate-io/terramate/generate"
)

func TestGenerateRootContext(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "empty generates empty file",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/empty.txt"),
						Expr("context", "root"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"empty.txt": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"empty.txt"},
					},
				},
			},
		},
		{
			name: "generate_file with false condition generates nothing",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "generate.context=root has access to project metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/source",
					add: Doc(
						GenerateFile(
							Labels("/stacks.txt"),
							Expr("context", "root"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{"stacks.txt"},
					},
				},
			},
		},
		{
			name: "generate.context=root fails when generating outside rootdir",
			configs: []hclconfig{
				{
					path: "/source",
					add: Doc(
						GenerateFile(
							Labels("/test/../../stacks.txt"),
							Expr("context", "root"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/"),
						},
						Error: errors.E(generate.ErrInvalidGenBlockLabel),
					},
				},
			},
		},
		{
			name: "generate.context=root with no stacks and accessing stacks.list",
			configs: []hclconfig{
				{
					path: "/source",
					add: Doc(
						GenerateFile(
							Labels("/stacks.txt"),
							Expr("context", "root"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`[]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{"stacks.txt"},
					},
				},
			},
		},
		{
			name: "generate files for all directories",
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateFile(
							Labels("/file1.txt"),
							Expr("context", "root"),
							Str("content", "/file1.txt"),
						),
						GenerateFile(
							Labels("/file2.txt"),
							Expr("context", "root"),
							Str("content", "/file2.txt"),
						),
					),
				},
				{
					path: "/nested/dir",
					add: Doc(
						GenerateFile(
							Labels("/target/dir/file1.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file1.txt"),
						),
						GenerateFile(
							Labels("/target/dir/file2.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file2.txt"),
						),
					),
				},
				{
					path: "/nested/stack",
					add: Doc(
						GenerateFile(
							Labels("/target/dir/file3.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file3.txt"),
						),
						GenerateFile(
							Labels("/target/dir/file4.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file4.txt"),
						),
						// It must also work if defined inside a stack.
						// Note(i4k): regression test.
						Stack(),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/file1.txt"),
						"file2.txt": stringer("/file2.txt"),
					},
				},
				{
					dir: "/target/dir",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/target/dir/file1.txt"),
						"file2.txt": stringer("/target/dir/file2.txt"),
						"file3.txt": stringer("/target/dir/file3.txt"),
						"file4.txt": stringer("/target/dir/file4.txt"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{"file1.txt", "file2.txt"},
					},
					{
						Dir:     project.NewPath("/target/dir"),
						Created: []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt"},
					},
				},
			},
		},
		{
			name: "generate_file.context=stack is ignored if not parent of any stack",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("file.txt"),
						Expr("context", "stack"),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "mixing generate_file.context",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("file.txt"),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content"),
					},
				},
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"file.txt"},
					},
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"file.txt"},
					},
				},
			},
		},
		{
			name: "generate.context=root is disallowed to generate inside stacks",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/stack/file.txt"),
						Str("content", "test"),
						Expr("context", "root"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stack"),
						},
						Error: errors.E(generate.ErrInvalidGenBlockLabel),
					},
				},
			},
		},
		{
			name: "generate.context=root must have absolute path in label",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("file.txt"),
						Str("content", "test"),
						Expr("context", "root"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/"),
						},
						Error: errors.E(generate.ErrInvalidGenBlockLabel),
					},
				},
			},
		},
		{
			name: "generate.context=root inside stack generating elsewhere",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/file.tm",
					add: GenerateFile(
						Labels("/target/empty.txt"),
						Expr("context", "root"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"empty.txt": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"empty.txt"},
					},
				},
			},
		},
		{
			name: "generate.context=root inside a subdir of stack generating elsewhere",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/subdir/dir/file.tm",
					add: GenerateFile(
						Labels("/target/empty.txt"),
						Expr("context", "root"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"empty.txt": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"empty.txt"},
					},
				},
			},
		},
		{
			name: "N generate_file with condition=false",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 2"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 3"),
					),
				},
			},
		},
		{
			name: "two generate_file with different condition (first is false)",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content 2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"file.txt"},
					},
				},
			},
		},
		{
			name: "two generate_file with different condition (first is true)",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content 1"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"file.txt"},
					},
				},
			},
		},
		{
			name: "multiple generate_file with same label and condition=true",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 2"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/target"),
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "multiple generate blocks with interleaving conditional blocks",
			configs: []hclconfig{
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 2"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 3"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file2.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 4"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 5"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 6"),
					),
				},
				{
					path: "/source",
					add: GenerateFile(
						Labels("/target/file3.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 7"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt":  stringer("content 1"),
						"file2.txt": stringer("content 4"),
						"file3.txt": stringer("content 7"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"file.txt", "file2.txt", "file3.txt"},
					},
				},
			},
		},
		{
			name: "child and parent directories with same label and different condition",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 1"),
					),
				},
				{
					path: "/child",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content 1"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/target"),
						Created: []string{"file.txt"},
					},
				},
			},
		},
		{
			name: "child and parent directories with same label and same condition - fails",
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 1"),
					),
				},
				{
					path: "/child",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 2"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/target"),
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
	})
}

func TestGenerateFileWithRootContextRemoveFilesWhenConditionIsFalse(t *testing.T) {
	t.Parallel()

	const filename = "file.txt"

	s := sandbox.NoGit(t, true)

	assertFileExist := func(file string) {
		t.Helper()

		path := filepath.Join(s.RootDir(), file)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("want file %q to exist, instead got: %v", path, err)
		}
	}
	assertFileDontExist := func(file string) {
		t.Helper()

		path := filepath.Join(s.RootDir(), file)
		_, err := os.Stat(path)

		if errors.Is(err, os.ErrNotExist) {
			return
		}

		t.Fatalf("want file %q to not exist, instead got: %v", path, err)
	}

	createConfig := func(filename string, condition bool) {
		s.RootEntry().CreateConfig(
			GenerateFile(
				Labels("/"+filename),
				Expr("context", "root"),
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
				Dir:     project.NewPath("/"),
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
				Dir:     project.NewPath("/"),
				Deleted: []string{filename},
			},
		},
	})
	assertFileDontExist(filename)
}

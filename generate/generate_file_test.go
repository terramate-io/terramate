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

package generate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestGenerateFile(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "no generate_file",
			layout: []string{
				"s:stack",
			},
		},
		{
			name: "dotfile is ignored",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/.test.tm",
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
					),
				},
			},
		},
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
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"empty": stringer(""),
					},
				},
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"empty": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"empty"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"empty"},
					},
				},
			},
		},
		{
			name: "simple plain string",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Str("content", "test"),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"test": stringer(`test`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name:   "all metadata available by default",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Expr("content", `<<EOT
stacks_list=${tm_jsonencode(terramate.stacks.list)}
stack_path_abs=${terramate.stack.path.absolute}
stack_path_rel=${terramate.stack.path.relative}
stack_path_to_root=${terramate.stack.path.to_root}
stack_path_basename=${terramate.stack.path.basename}
stack_id=${tm_try(terramate.stack.id, "no-id")}
stack_name=${terramate.stack.name}
stack_description=${terramate.stack.description}
EOT`,
						)),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"test": stringer(`stacks_list=["/stack"]
stack_path_abs=/stack
stack_path_rel=stack
stack_path_to_root=..
stack_path_basename=stack
stack_id=no-id
stack_name=stack
stack_description=`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "stack.id metadata available",
			layout: []string{
				"s:stack:id=stack-id",
			},
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Expr("content", `<<EOT

stack_id=${terramate.stack.id}
EOT`,
						)),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"test": stringer(
							`stack_id=stack-id`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name:   "using globals and metadata with interpolation",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: Globals(
						Str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Str("content", "${global.data}-${terramate.stack.path.absolute}"),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"test": stringer("global-data-/stack"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name:   "mixed conditions on different blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: Doc(
						GenerateFile(
							Labels("test"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("test2"),
							Bool("condition", true),
							Str("content", "data"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"test2": stringer("data"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"test2"},
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
			name: "terramate.stacks.list with root workdir",
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
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"stacks.txt"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"stacks.txt"},
					},
				},
			},
		},
		{
			name: "terramate.stacks.list with stack workdir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-1",
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
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"stacks.txt"},
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
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-1"),
						"file2.txt": stringer("stack-1"),
					},
				},
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-2"),
						"file2.txt": stringer("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"file1.txt", "file2.txt"},
					},
					{
						Dir:     "/stacks/stack-2",
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
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/stacks/stack-2"),
						"file2.txt": stringer("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-2",
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
				Dir:     "/stack",
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
				Dir:     "/stack",
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
				Dir:     "/stack",
				Created: []string{generatedFile},
			},
		},
	})

	want := s.RootDir() + "-" + filepath.Base(s.RootDir())
	got := stackEntry.ReadFile(generatedFile)

	assert.EqualStrings(t, want, got)
}

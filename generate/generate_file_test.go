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

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestGenerateFile(t *testing.T) {
	checkGenFiles := func(t *testing.T, got string, want string) {
		t.Helper()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Error("generated file doesn't match expectation")
			t.Errorf("want:\n%q", want)
			t.Errorf("got:\n%q", got)
			t.Fatalf("diff:\n%s", diff)
		}
	}
	testCodeGeneration(t, checkGenFiles, []testcase{
		{
			name: "empty generate_file content generates nothing",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: generateFile(
						labels("empty"),
						str("content", ""),
					),
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
					add: generateFile(
						labels("test"),
						boolean("condition", false),
						str("content", "content"),
					),
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
					add: hcldoc(
						generateFile(
							labels("file1.txt"),
							expr("content", "terramate.path"),
						),
						generateFile(
							labels("file2.txt"),
							expr("content", "terramate.name"),
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
						StackPath: "/stacks/stack-1",
						Created:   []string{"file1.txt", "file2.txt"},
					},
					{
						StackPath: "/stacks/stack-2",
						Created:   []string{"file1.txt", "file2.txt"},
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
					add: hcldoc(
						generateFile(
							labels("file1.txt"),
							expr("content", "terramate.path"),
						),
						generateFile(
							labels("file2.txt"),
							expr("content", "terramate.name"),
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
						StackPath: "/stacks/stack-2",
						Created:   []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			// TODO(katcipis): define a proper behavior where
			// directories are allowed but in a constrained fashion.
			// This is a quick fix to avoid creating files on arbitrary
			// places around the file system.
			name: "generate file with dir separators on label name fails",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"s:stacks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-1",
					add: generateFile(
						labels("/name"),
						str("content", "something"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: generateFile(
						labels("./name"),
						str("content", "something"),
					),
				},
				{
					path: "/stacks/stack-3",
					add: generateFile(
						labels("./dir/name"),
						str("content", "something"),
					),
				},
				{
					path: "/stacks/stack-4",
					add: generateFile(
						labels("dir/name"),
						str("content", "something"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-1",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-2",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-3",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-4",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
				},
			},
		},
	})
}

func TestGenerateFileRemoveFilesWhenConditionIsFalse(t *testing.T) {
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
			stackConfig(
				generateFile(
					labels(filename),
					boolean("condition", condition),
					str("content", "some content"),
				),
			).String())
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
				StackPath: "/stack",
				Created:   []string{filename},
			},
		},
	})
	assertFileExist(filename)

	createConfig(filename, false)
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				StackPath: "/stack",
				Deleted:   []string{filename},
			},
		},
	})
	assertFileDontExist(filename)
}

func TestGenerateFileTerramateMetadata(t *testing.T) {
	// We need to know the sandbox abspath to test terramate.root properly
	const generatedFile = "file.hcl"

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	s.RootEntry().CreateConfig(
		hcldoc(
			generateFile(
				labels(generatedFile),
				expr("content", "terramate.root.path.absolute"),
			),
		).String(),
	)

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				StackPath: "/stack",
				Created:   []string{generatedFile},
			},
		},
	})

	want := s.RootDir()
	got := stackEntry.ReadFile(generatedFile)

	assert.EqualStrings(t, want, got)
}

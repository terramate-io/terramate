// Copyright 2021 Mineiros GmbH
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
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	stackpkg "github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

// TODO(katcipis): if subdir is stack fails
// TODO(katcipis): if subdir is symlink fails

type (
	stringer string

	generatedFile struct {
		stack string
		files map[string]fmt.Stringer
	}
	testcase struct {
		name       string
		layout     []string
		configs    []hclconfig
		workingDir string
		want       []generatedFile
		wantReport generate.Report
	}

	hclconfig struct {
		path string
		add  fmt.Stringer
	}
)

func (s stringer) String() string {
	return string(s)
}

func TestGeneratePathOnLabels(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "subdirs with no relative walk are allowed",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("dir/file.hcl"),
						Content(
							Block("block",
								Str("data", "data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateFile(
						Labels("dir/sub/file.txt"),
						Str("content", "test"),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Block("block",
								Str("data", "data"),
							),
						),
						"dir/sub/file.txt": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/stacks/stack",
						Created: []string{
							"dir/file.hcl",
							"dir/sub/file.txt",
						},
					},
				},
			},
		},
		{
			name: "if path is stack fails",
			layout: []string{
				"s:stacks/stack",
				"s:stacks/stack/child-stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("child-stack/name.tf"),
							Content(
								Block("something"),
							),
						),

						GenerateFile(
							Labels("child-stack/name.txt"),
							Str("content", "something"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack/child-stack",
					files: map[string]fmt.Stringer{
						"child-stack/name.tf": Doc(
							Block("something"),
						),
						"child-stack/name.txt": stringer("something"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/stacks/stack/child-stack",
						Created: []string{
							"child-stack/name.tf",
							"child-stack/name.txt",
						},
					},
				},
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.L(
							errors.E(generate.ErrInvalidGenBlockLabel),
							errors.E(generate.ErrInvalidGenBlockLabel),
						),
					},
				},
			},
		},
		{
			name: "invalid paths fails",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"s:stacks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-1",
					add: Doc(
						GenerateHCL(
							Labels("/name.tf"),
							Content(
								Block("something"),
							),
						),

						GenerateFile(
							Labels("/name.txt"),
							Str("content", "something"),
						),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Doc(
						GenerateHCL(
							Labels("../name.tf"),
							Content(
								Block("something"),
							),
						),
						GenerateFile(
							Labels("../name.txt"),
							Str("content", "something"),
						),
					),
				},
				{
					path: "/stacks/stack-3",
					add: Doc(
						GenerateHCL(
							Labels("a/b/../../../name.tf"),
							Content(
								Block("something"),
							),
						),
						GenerateFile(
							Labels("a/b/../../../name.txt"),
							Str("content", "something"),
						),
					),
				},
				{
					path: "/stacks/stack-4",
					add: Doc(
						GenerateHCL(
							Labels("./name.tf"),
							Content(
								Block("something"),
							),
						),
						GenerateFile(
							Labels("./name.txt"),
							Str("content", "something"),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack-1",
						},
						Error: errors.L(
							errors.E(generate.ErrInvalidGenBlockLabel),
							errors.E(generate.ErrInvalidGenBlockLabel),
						),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-2",
						},
						Error: errors.L(
							errors.E(generate.ErrInvalidGenBlockLabel),
							errors.E(generate.ErrInvalidGenBlockLabel),
						),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-3",
						},
						Error: errors.L(
							errors.E(generate.ErrInvalidGenBlockLabel),
							errors.E(generate.ErrInvalidGenBlockLabel),
						),
					},
					{
						Result: generate.Result{
							Dir: "/stacks/stack-4",
						},
						Error: errors.L(
							errors.E(generate.ErrInvalidGenBlockLabel),
							errors.E(generate.ErrInvalidGenBlockLabel),
						),
					},
				},
			},
		},
	})
}

func TestGenerateConflictsBetweenGenerateTypes(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "stack with different generate blocks but same label",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateFile(
						Labels("repeated"),
						Str("content", "test"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "stack with block with same label as parent but different condition",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
						Bool("condition", false),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateFile(
						Labels("repeated"),
						Str("content", "test"),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"repeated": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack",
						Created: []string{"repeated"},
					},
				},
			},
		},
		{
			name: "stack with different generate blocks and same label but different condition",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
						Bool("condition", false),
					),
				},
				{
					path: "/stack",
					add: GenerateFile(
						Labels("repeated"),
						Str("content", "test"),
						Bool("condition", true),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stack",
					files: map[string]fmt.Stringer{
						"repeated": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"repeated"},
					},
				},
			},
		},
	})
}

func testCodeGeneration(t *testing.T, tcases []testcase) {
	t.Helper()

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Helper()
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.DefaultFilename, cfg.add.String())
			}

			assertGeneratedFiles := func(t *testing.T) {
				t.Helper()

				for _, wantDesc := range tcase.want {
					stackRelPath := wantDesc.stack[1:]
					stack := s.StackEntry(stackRelPath)

					for name, wantFiles := range wantDesc.files {
						want := wantFiles.String()
						got := stack.ReadFile(name)

						test.AssertGenCodeEquals(t, got, want)
					}
				}
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			report := generate.Do(s.RootDir(), workingDir)
			assertEqualReports(t, report, tcase.wantReport)

			assertGeneratedFiles(t)

			// piggyback on the tests to validate that regeneration doesn't
			// delete files or fail and has identical results.
			t.Run("regenerate", func(t *testing.T) {
				report := generate.Do(s.RootDir(), workingDir)
				// since we just generated everything, report should only contain
				// the same failures as previous code generation.
				assertEqualReports(t, report, generate.Report{
					Failures: tcase.wantReport.Failures,
				})
				assertGeneratedFiles(t)
			})

			// Check we don't have extraneous/unwanted files
			// We remove wanted/expected generated code
			// So we should have only basic terramate configs left
			// There is potential to extract this for other code generation tests.
			for _, wantDesc := range tcase.want {
				stackRelPath := wantDesc.stack[1:]
				stack := s.StackEntry(stackRelPath)
				for name := range wantDesc.files {
					stack.RemoveFile(name)
				}
			}
			err := filepath.WalkDir(s.RootDir(), func(path string, d fs.DirEntry, err error) error {
				t.Helper()

				assert.NoError(t, err, "checking for unwanted generated files")
				if d.IsDir() {
					if d.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}

				// sandbox creates README.md inside test dirs
				if d.Name() == config.DefaultFilename ||
					d.Name() == stackpkg.DefaultFilename ||
					d.Name() == "README.md" {
					return nil
				}

				t.Errorf("unwanted file %q", path)
				return nil
			})

			assert.NoError(t, err)
		})
	}
}

func assertEqualStringList(t *testing.T, got []string, want []string) {
	t.Helper()

	assert.EqualInts(t, len(want), len(got), "want %+v != got %+v", want, got)
	failed := false

	for i, wv := range want {
		gv := got[i]
		if gv != wv {
			failed = true
			t.Errorf("got[%d][%s] != want[%d][%s]", i, gv, i, wv)
		}
	}

	if failed {
		t.Fatalf("got %v != want %v", got, want)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

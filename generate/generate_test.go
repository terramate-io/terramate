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

type (
	stringer string

	genFileChecker func(*testing.T, string, string)

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

func TestGenerateConflictsBetweenGenerateTypes(t *testing.T) {
	testCodeGeneration(t, test.AssertGenHCLEquals, []testcase{
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
							StackPath: "/stacks/stack",
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
						StackPath: "/stacks/stack",
						Created:   []string{"repeated"},
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
						StackPath: "/stack",
						Created:   []string{"repeated"},
					},
				},
			},
		},
	})
}

func testCodeGeneration(t *testing.T, checkGenFile genFileChecker, tcases []testcase) {
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

						checkGenFile(t, got, want)
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
	for i, wv := range want {
		gv := got[i]
		if gv != wv {
			t.Errorf("got[%d][%s] != want[%d][%s]", i, gv, i, wv)
		}
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

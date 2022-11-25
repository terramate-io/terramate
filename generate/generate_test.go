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
	"runtime"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/project"
	stackpkg "github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

type (
	stringer string

	generatedFile struct {
		dir   string
		files map[string]fmt.Stringer
	}
	testcase struct {
		name       string
		skipOn     string
		layout     []string
		configs    []hclconfig
		vendorDir  string
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

func TestGenerateIgnore(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "dir with skip file",
			layout: []string{
				"s:stacks/stack",
				"s:stacks/stack-2",
				"f:stacks/stack-2/" + config.SkipFilename,
				"f:not-a-stack/" + config.SkipFilename,
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("file.hcl"),
						Content(
							Block("block",
								Str("data", "data"),
							),
						),
					),
				},
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
					path: "/not-a-stack",
					add: GenerateFile(
						Labels("/target/file.hcl"),
						Expr("context", "root"),
						Str("content", "data"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							Block("block",
								Str("data", "data"),
							),
						),
						"dir/file.hcl": Doc(
							Block("block",
								Str("data", "data"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/stacks/stack",
						Created: []string{
							"dir/file.hcl",
							"file.hcl",
						},
					},
				},
			},
		},
	})
}

func TestGenerateStackContextSubDirsOnLabels(t *testing.T) {
	t.Parallel()

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
						Labels("file.hcl"),
						Content(
							Block("block",
								Str("data", "data"),
							),
						),
					),
				},
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
					dir: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							Block("block",
								Str("data", "data"),
							),
						),
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
							"file.hcl",
						},
					},
				},
			},
		},
		{
			name: "if path is a child stack fails",
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
					dir: "/stacks/stack/child-stack",
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
			name: "if path is inside child stack fails",
			layout: []string{
				"s:stacks/stack",
				"s:stacks/stack/child-stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("child-stack/dir/name.tf"),
							Content(
								Block("something"),
							),
						),

						GenerateFile(
							Labels("child-stack/dir/name.txt"),
							Str("content", "something"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack/child-stack",
					files: map[string]fmt.Stringer{
						"child-stack/dir/name.tf": Doc(
							Block("something"),
						),
						"child-stack/dir/name.txt": stringer("something"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/stacks/stack/child-stack",
						Created: []string{
							"child-stack/dir/name.tf",
							"child-stack/dir/name.txt",
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
			name:   "if path is symlink fails",
			skipOn: "windows",
			layout: []string{
				"s:stacks/stack",
				"d:somedir",
				"l:somedir:stacks/stack/symlink",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: Doc(
						GenerateHCL(
							Labels("symlink/name.tf"),
							Content(
								Block("something"),
							),
						),
						GenerateFile(
							Labels("symlink/name.txt"),
							Str("content", "something"),
						),
					),
				},
			},
			wantReport: generate.Report{
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

func TestGenerateCleanup(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "deletes generated files outside stacks",
			layout: []string{
				genfile("dir/a.hcl"),
				genfile("dir/subdir/b.hcl"),
				genfile("dir/subdir/again/c.hcl"),
				genfile("another/d.hcl"),
				genfile("root.hcl"),
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/",
						Deleted: []string{
							"root.hcl",
						},
					},
					{
						Dir:     "/another",
						Deleted: []string{"d.hcl"},
					},
					{
						Dir:     "/dir",
						Deleted: []string{"a.hcl"},
					},
					{
						Dir:     "/dir/subdir",
						Deleted: []string{"b.hcl"},
					},
					{
						Dir:     "/dir/subdir/again",
						Deleted: []string{"c.hcl"},
					},
				},
			},
		},
		{
			name: "each stack owns cleanup of all its subdirs",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-1/stack-1-a",
				"s:stacks/stack-1/stack-1-b",
				genfile("dir/orphan.hcl"),
				genfile("stacks/stack-1/a.hcl"),
				genfile("stacks/stack-1/subdir/b.hcl"),
				genfile("stacks/stack-1/subdir/dir/c.hcl"),
				genfile("stacks/stack-1/stack-1-a/e.hcl"),
				genfile("stacks/stack-1/stack-1-a/subdir/f.hcl"),
				genfile("stacks/stack-1/stack-1-a/subdir/dir/g.hcl"),
				genfile("stacks/stack-1/stack-1-b/h.hcl"),
				genfile("stacks/stack-2/d.hcl"),
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: "/dir",
						Deleted: []string{
							"orphan.hcl",
						},
					},
					{
						Dir: "/stacks/stack-1",
						Deleted: []string{
							"a.hcl",
							"subdir/b.hcl",
							"subdir/dir/c.hcl",
						},
					},
					{
						Dir: "/stacks/stack-1/stack-1-a",
						Deleted: []string{
							"e.hcl",
							"subdir/dir/g.hcl",
							"subdir/f.hcl",
						},
					},
					{
						Dir: "/stacks/stack-1/stack-1-b",
						Deleted: []string{
							"h.hcl",
						},
					},
					{
						Dir: "/stacks/stack-2",
						Deleted: []string{
							"d.hcl",
						},
					},
				},
			},
		},
	})
}

func TestGenerateCleanupFailsToReadFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	dir := s.RootEntry().CreateDir("dir")
	file := dir.CreateFile("file.hcl", genhcl.Header)
	file.Chmod(0)
	defer file.Chmod(0755)

	report := s.Generate()
	t.Log(report)
	assert.EqualInts(t, 0, len(report.Successes), "want no successes")
	assert.EqualInts(t, 0, len(report.Failures), "want no failures")
	assert.Error(t, report.CleanupErr)
}

func TestGenerateConflictsBetweenGenerateTypes(t *testing.T) {
	t.Parallel()

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
					dir: "/stacks/stack",
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
					dir: "/stack",
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

	for _, tc := range tcases {
		tcase := tc

		t.Run(tcase.name, func(t *testing.T) {
			t.Helper()

			if tcase.skipOn == runtime.GOOS {
				t.Skipf("skipping on GOOS %q", tcase.skipOn)
				return
			}

			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.DefaultFilename, cfg.add.String())
			}

			assertGeneratedFiles := func(t *testing.T) {
				t.Helper()

				for _, wantDesc := range tcase.want {
					stackRelPath := wantDesc.dir[1:]
					stack := s.StackEntry(stackRelPath)

					for name, wantFiles := range wantDesc.files {
						want := wantFiles.String()
						got := stack.ReadFile(name)

						test.AssertGenCodeEquals(t, got, want)
					}
				}
			}

			vendorDir := project.NewPath("/modules")
			if tcase.vendorDir != "" {
				vendorDir = project.NewPath(tcase.vendorDir)
			}
			report := generate.Do(s.Config(), vendorDir, nil)
			assertEqualReports(t, report, tcase.wantReport)

			assertGeneratedFiles(t)

			// piggyback on the tests to validate that regeneration doesn't
			// delete files or fail and has identical results.
			t.Run("regenerate", func(t *testing.T) {
				report := generate.Do(s.Config(), vendorDir, nil)
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
				stackRelPath := wantDesc.dir[1:]
				stack := s.StackEntry(stackRelPath)
				for name := range wantDesc.files {
					stack.RemoveFile(name)
				}
			}

			createdBySandbox := func(path string) bool {
				relpath := strings.TrimPrefix(path, s.RootDir())
				// For windows compatibility, since builder strings
				// are unix like.
				relpath = filepath.ToSlash(relpath)
				relpath = strings.TrimPrefix(relpath, "/")
				for _, builder := range tcase.layout {
					switch {
					case strings.HasPrefix(builder, "f:"):
						buildPath := builder[2:]
						if buildPath == relpath {
							return true
						}
					case strings.HasPrefix(builder, "l:"):
						c := strings.Split(builder, ":")
						linkPath := c[2]
						if linkPath == relpath {
							return true
						}
					}
				}
				return false
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

				if createdBySandbox(path) {
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

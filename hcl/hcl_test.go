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

package hcl_test

import (
	"fmt"
	"path/filepath"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/rs/zerolog"
	"github.com/zclconf/go-cty-debug/ctydebug"
)

type (
	cfgWant struct {
		errs   []error
		config hcl.Config
	}

	globalsWant struct {
		errs    []error
		globals *hclwrite.Block
	}

	cfgfile struct {
		filename string
		body     string
	}

	cfgTestcase struct {
		name     string
		parsedir string
		rootdir  string
		input    []cfgfile
		want     cfgWant
	}

	globalsTestcase struct {
		name     string
		parsedir string
		rootdir  string
		input    []cfgfile
		want     globalsWant
	}
)

func TestHCLParserTerramateBlock(t *testing.T) {
	for _, tc := range []cfgTestcase{
		{
			name: "unrecognized blocks",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     "something {}\nsomething_else {}",
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(1, 1, 0), end(1, 12, 11))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(2, 1, 13), end(2, 17, 29))),
				},
			},
		},
		{
			name: "unrecognized attribute",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate{}
						something = 1
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 7, 25), end(3, 16, 34))),
				},
			},
		},
		{
			name: "unrecognized attribute inside terramate block",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate{
							something = 1
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 8, 25), end(3, 17, 34))),
				},
			},
		},
		{
			name: "unrecognized terramate block",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `terramate{
							something {}
							other {}
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(2, 8, 18), end(2, 19, 29))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 8, 38), end(3, 15, 45))),
				},
			},
		},
		{
			name: "multiple empty terramate blocks on same file",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate{}
						terramate{}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{Terramate: &hcl.Terramate{}},
			},
		},
		{
			name: "conflicting terramate.required_version attributes",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate{
							required_version = "= 1.0"
						}
						terramate{
							required_version = "= 2.0"
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(6, 8, 84), end(6, 24, 100))),
				},
			},
		},
		{
			name: "empty config",
			want: cfgWant{
				config: hcl.Config{},
			},
		},
		{
			name: "invalid version",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							required_version = 1
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 27, 45), end(3, 28, 46))),
				},
			},
		},
		{
			name: "interpolation not allowed at req_version",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							required_version = "${test.version}"
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 30, 48), end(3, 34, 52))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 27, 45), end(3, 44, 62))),
				},
			},
		},
		{
			name: "invalid attributes",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							version = 1
							invalid = 2
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(3, 8, 26), end(3, 15, 33))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(4, 8, 45), end(4, 15, 52))),
				},
			},
		},
		{
			name: "required_version > 0.0.0",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						       required_version = "> 0.0.0"
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "> 0.0.0",
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserRootConfig(t *testing.T) {
	for _, tc := range []cfgTestcase{
		{
			name: "no config returns empty config",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     `terramate {}`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
				},
			},
		},
		{
			name: "empty config block returns empty config",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{},
					},
				},
			},
		},
		{
			name: "unrecognized config attribute",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								something = "bleh"
							}
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "unrecognized config.git field",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					terramate {
						config {
							git {
								test = 1
							}
						}
					}
				`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(5, 9, 54), end(5, 13, 58))),
				},
			},
		},
		{
			name: "empty config.git block",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								git {}
							}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple empty config blocks",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {}
							config {}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{},
					},
				},
			},
		},
		{
			name: "basic config.git block",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:    "trunk",
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "all fields set for config.git",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								git {
									default_branch          = "trunk"
									default_remote          = "upstream"
									default_branch_base_ref = "HEAD~2"
									check_untracked         = false
									check_uncommitted       = false
									check_remote            = false
								}
							}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:        "trunk",
								DefaultRemote:        "upstream",
								DefaultBranchBaseRef: "HEAD~2",
								CheckUntracked:       false,
								CheckUncommitted:     false,
								CheckRemote:          false,
							},
						},
					},
				},
			},
		},
		{
			name: "git.check fields default to true if git block is present",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								git {}
							}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "git.check fields must be boolean",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
							config {
								git {
									check_untracked   = "hi"
									check_uncommitted = 666
									check_remote      = []
								}
							}
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(5, 30, 78), end(5, 34, 82))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(6, 30, 112), end(6, 33, 115))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg.tm", start(7, 30, 145), end(7, 32, 147))),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserMultipleErrors(t *testing.T) {
	for _, tc := range []cfgTestcase{
		{
			name: "multiple syntax errors",
			input: []cfgfile{
				{
					filename: "file.tm",
					body:     "a=1\na=2\na=3",
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file.tm", start(2, 1, 4), end(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file.tm", start(3, 1, 8), end(3, 2, 9))),
				},
			},
		},
		{
			name: "multiple syntax errors in different files",
			input: []cfgfile{
				{
					filename: "file1.tm",
					body:     "a=1\na=2\na=3",
				},
				{
					filename: "file2.tm",
					body:     "a=1\na=2\na=3",
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file1.tm", start(2, 1, 4), end(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file1.tm", start(3, 1, 8), end(3, 2, 9))),
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file2.tm", start(2, 1, 4), end(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						mkrange("file2.tm", start(3, 1, 8), end(3, 2, 9))),
				},
			},
		},
		{
			name: "conflicting stack files",
			input: []cfgfile{
				{
					filename: "stack1.tm",
					body:     "stack {}",
				},
				{
					filename: "stack2.tm",
					body:     "stack {}",
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("stack2.tm", start(1, 1, 0), end(1, 8, 7))),
				},
			},
		},
		{
			name: "conflicting terramate git config and other errors",
			input: []cfgfile{
				{
					filename: "cfg1.tm",
					body: `terramate {
						config {
							git {
								default_branch = "trunk"
							}
						}
					}
					
					test {}`,
				},
				{
					filename: "cfg2.tm",
					body: `terramate {
						config {
							git {
								default_branch = "test"
							}
						}
					}`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg1.tm", start(9, 6, 108), end(9, 12, 114))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg2.tm", start(4, 9, 48), end(4, 23, 62))),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserTerramateBlocksMerging(t *testing.T) {
	tcases := []cfgTestcase{
		{
			name: "two config file with terramate blocks",
			input: []cfgfile{
				{
					filename: "version.tm",
					body: `
						terramate {
							required_version = "0.0.1"
						}
					`,
				},
				{
					filename: "config.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "0.0.1",
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:    "trunk",
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "three config files with terramate and stack blocks",
			input: []cfgfile{
				{
					filename: "version.tm",
					body: `
						terramate {
							required_version = "6.6.6"
						}
					`,
				},
				{
					filename: "config.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
				{
					filename: "stack.tm",
					body: `
						stack {
							name = "stack"
							description = "some stack"
							after = ["after"]
							before = ["before"]
							wants = ["wants"]
							watch = ["watch"]
						}
					`,
				},
			},
			want: cfgWant{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "6.6.6",
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:    "trunk",
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
					Stack: &hcl.Stack{
						Name:        "stack",
						Description: "some stack",
						After:       []string{"after"},
						Before:      []string{"before"},
						Wants:       []string{"wants"},
						Watch:       []string{"watch"},
					},
				},
			},
		},
		{
			name: "multiple files with stack blocks fail",
			input: []cfgfile{
				{
					filename: "stack_name.tm",
					body: `
						stack {
							name = "stack"
						}
					`,
				},
				{
					filename: "stack_desc.tm",
					body: `
						stack {
							description = "some stack"
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "multiple files with conflicting terramate.config.git attributes fail",
			input: []cfgfile{
				{
					filename: "git.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
				{
					filename: "gitagain.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "other"
								}
							}
						}
					`,
				},
			},
			want: cfgWant{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
	}

	for _, tc := range tcases {
		testParser(t, tc)
	}
}

func prepareParserTests(
	t *testing.T,
	wantErrs []error,
	files []cfgfile,
	rootdir string,
	parsedir string,
) (string, string) {
	t.Helper()

	configsDir := t.TempDir()
	for _, inputConfigFile := range files {
		if inputConfigFile.filename == "" {
			panic("expect a filename in the input config")
		}
		path := filepath.Join(configsDir, inputConfigFile.filename)
		dir := filepath.Dir(path)
		filename := filepath.Base(path)
		test.WriteFile(t, dir, filename, inputConfigFile.body)
	}
	fixupFiledirOnErrorsFileRanges(configsDir, wantErrs)

	if parsedir == "" {
		parsedir = configsDir
	} else {
		parsedir = filepath.Join(configsDir, parsedir)
	}

	if rootdir == "" {
		rootdir = configsDir
	} else {
		rootdir = filepath.Join(configsDir, rootdir)
	}
	return rootdir, parsedir
}

func testParser(t *testing.T, tc cfgTestcase) {
	t.Run(tc.name, func(t *testing.T) {
		rootdir, parsedir := prepareParserTests(
			t, tc.want.errs, tc.input, tc.rootdir, tc.parsedir,
		)
		got, err := hcl.ParseDir(rootdir, parsedir)
		errtest.AssertErrorList(t, err, tc.want.errs)

		var gotErrs *errors.List
		if errors.As(err, &gotErrs) {
			if len(gotErrs.Errors()) != len(tc.want.errs) {
				t.Logf("got errors: %s", gotErrs.Detailed())
				t.Fatalf("got %d errors but want %d",
					len(gotErrs.Errors()), len(tc.want.errs))
			}
		}

		if tc.want.errs == nil {
			test.AssertTerramateConfig(t, got, tc.want.config)
		}
	})

	// This is a lazy way to piggyback on our current set of tests
	// to test that we have identical behavior when configuration
	// is on a different file that is not Terramate default.
	// Old tests don't inform a specific filename (assuming default).
	// We use this to test each of these simple scenarios with
	// different filenames (other than default).
	if len(tc.input) != 1 || tc.input[0].filename != "" {
		return
	}

	validConfigFilenames := []string{
		"config.tm",
		"config.tm.hcl",
	}

	for _, filename := range validConfigFilenames {
		newtc := cfgTestcase{
			name: fmt.Sprintf("%s with filename %s", tc.name, filename),
			input: []cfgfile{
				{
					filename: filename,
					body:     tc.input[0].body,
				},
			},
			want: tc.want,
		}
		testParser(t, newtc)
	}
}

func TestParserLoadGlobals(t *testing.T) {
	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("globals", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		return hclwrite.AttributeValue(t, name, expr)
	}

	for _, tc := range []globalsTestcase{
		{
			name:     "globals can reference imported values",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/imported.tm"
					}
					globals {
						B = "redefined from ${global.A}"
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `
						globals {
							A = "imported"
						}
					`,
				},
			},
			want: globalsWant{
				globals: globals(
					attr("B", `"redefined from imported"`),
					attr("A", `"imported"`),
				),
			},
		},
		{
			name:     "imported files are handled before importing file",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `
					globals {
						B = "redefined from ${global.A}"
					}

					import {
						source = "/other/imported.tm"
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `
						globals {
							A = "other/imported.tm"
						}
					`,
				},
			},
			want: globalsWant{
				globals: globals(
					attr("B", `"redefined from other/imported.tm"`),
					attr("A", `"other/imported.tm"`),
				),
			},
		},
		{
			name:     "imported file has redefinition of own imports",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `
					import {
						source = "/other/imported.tm"
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `
						globals {
							A = "${global.B}"
						}

						import {
							source = "/other2/imported.tm"
						}
					`,
				},
				{
					filename: "other2/imported.tm",
					body: `
						globals {
							B = "other2/imported.tm"
						}
					`,
				},
			},
			want: globalsWant{
				globals: globals(
					attr("A", `"other2/imported.tm"`),
					attr("B", `"other2/imported.tm"`),
				),
			},
		},
		{
			name:     "multiple level redefinition",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `
					globals {
						A = global.B
					}
					import {
						source = "/other/imported.tm"
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `
						globals {
							B = global.C
						}

						import {
							source = "/other2/imported.tm"
						}
					`,
				},
				{
					filename: "other2/imported.tm",
					body: `
						globals {
							C = "defined at other2/imported.tm"
						}
					`,
				},
			},
			want: globalsWant{
				globals: globals(
					attr("A", `"defined at other2/imported.tm"`),
					attr("B", `"defined at other2/imported.tm"`),
					attr("C", `"defined at other2/imported.tm"`),
				),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootdir, stackdir := prepareParserTests(
				t, tc.want.errs, tc.input, tc.rootdir, tc.parsedir,
			)

			test.WriteFile(t, stackdir, "stack.tm", `stack {}`)

			st, err := stack.Load(rootdir, stackdir)
			assert.NoError(t, err)

			gotGlobals, err := stack.LoadGlobals(rootdir, st)
			errtest.AssertErrorList(t, err, tc.want.errs)

			var gotErrs *errors.List
			if errors.As(err, &gotErrs) {
				if len(gotErrs.Errors()) != len(tc.want.errs) {
					t.Logf("got errors: %s", gotErrs.Detailed())
					t.Fatalf("got %d errors but want %d",
						len(gotErrs.Errors()), len(tc.want.errs))
				}
			}

			if tc.want.errs != nil {
				return
			}

			if tc.want.globals.HasExpressions() {
				t.Fatal("wanted globals definition contains expressions, they should be defined only by evaluated values")
				t.Errorf("wanted globals definition:\n%s\n", tc.want.globals)
			}

			want := tc.want.globals

			gotAttrs := gotGlobals.Attributes()
			wantAttrs := want.AttributesValues()

			if len(gotAttrs) != len(wantAttrs) {
				t.Errorf("got %d global attributes; wanted %d", len(gotAttrs), len(wantAttrs))
			}

			for name, wantVal := range wantAttrs {
				gotVal, ok := gotAttrs[name]
				if !ok {
					t.Errorf("wanted global.%s is missing", name)
					continue
				}
				if diff := ctydebug.DiffValues(wantVal, gotVal); diff != "" {
					t.Errorf("global.%s doesn't match expectation", name)
					t.Errorf("want: %s", ctydebug.ValueString(wantVal))
					t.Errorf("got: %s", ctydebug.ValueString(gotVal))
					t.Errorf("diff:\n%s", diff)
				}
			}
		})
	}
}

func TestHCLParseReParsingFails(t *testing.T) {
	temp := t.TempDir()
	p, err := hcl.NewTerramateParser(temp, temp)
	assert.NoError(t, err)
	test.WriteFile(t, temp, "test.tm", `terramate {}`)
	err = p.AddDir(temp)
	assert.NoError(t, err)
	_, err = p.ParseConfig()
	assert.NoError(t, err)

	_, err = p.ParseConfig()
	assert.Error(t, err)
	err = p.Parse()
	assert.Error(t, err)
}

func TestHCLParseProvidesAllParsedBodies(t *testing.T) {
	cfgdir := t.TempDir()
	parser, err := hcl.NewTerramateParser(cfgdir, cfgdir)
	assert.NoError(t, err)

	const filename = "stack.tm"

	test.WriteFile(t, cfgdir, filename, `
		stack {}

		generate_hcl "file.tf" {
			content {}
		}

		generate_file "file.txt" {
			content = ""
		}

		globals {
			a = "hi"
		}
	`)

	err = parser.AddDir(cfgdir)
	assert.NoError(t, err)

	_, err = parser.ParseConfig()
	assert.NoError(t, err)

	cfgpath := filepath.Join(cfgdir, filename)
	bodies := parser.ParsedBodies()
	body, ok := bodies[cfgpath]

	assert.IsTrue(t, ok, "unable to find body for cfg %q on bodies: %v", cfgpath, bodies)
	assert.EqualInts(t, 0, len(body.Attributes), "want 0 parsed attributes, got: %d", len(body.Attributes))
	assert.EqualInts(t, 4, len(body.Blocks), "want 4 parsed blocks, got: %d", len(body.Blocks))

	blocks := body.Blocks
	assert.EqualStrings(t, "stack", blocks[0].Type)
	assert.EqualStrings(t, "generate_hcl", blocks[1].Type)
	assert.EqualStrings(t, "generate_file", blocks[2].Type)
	assert.EqualStrings(t, "globals", blocks[3].Type)
}

// some helpers to easy build file ranges.
func mkrange(fname string, start, end hhcl.Pos) hhcl.Range {
	return hhcl.Range{
		Filename: fname,
		Start:    start,
		End:      end,
	}
}

func start(line, column, char int) hhcl.Pos {
	return hhcl.Pos{
		Line:   line,
		Column: column,
		Byte:   char,
	}
}

func fixupFiledirOnErrorsFileRanges(dir string, errs []error) {
	for _, err := range errs {
		if e, ok := err.(*errors.Error); ok {
			e.FileRange.Filename = filepath.Join(dir, e.FileRange.Filename)
		}
	}
}

var end = start

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

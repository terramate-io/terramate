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
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/rs/zerolog"
)

type (
	want struct {
		errs   []error
		config hcl.Config
	}

	cfgfile struct {
		filename string
		body     string
	}

	testcase struct {
		name  string
		input []cfgfile
		want  want
	}
)

func TestHCLParserModules(t *testing.T) {
	type want struct {
		modules []hcl.Module
		errs    []error
	}
	type testcase struct {
		name  string
		input cfgfile
		want  want
	}

	for _, tc := range []testcase{
		{
			name: "module must have 1 label",
			input: cfgfile{
				filename: "main.tf",
				body:     `module {}`,
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerraformSchema,
					mkrange("main.tf", start(1, 8, 7), end(1, 9, 8)))},
			},
		},
		{
			name: "module must have a source attribute",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "test" {}`,
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerraformSchema,
					mkrange("main.tf", start(1, 15, 14), end(1, 17, 16)))},
			},
		},
		{
			name: "empty source is a valid module",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "test" {source = ""}`,
			},
			want: want{
				modules: []hcl.Module{
					{
						Source: "",
					},
				},
			},
		},
		{
			name: "valid module",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "test" {source = "test"}`,
			},
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
				},
			},
		},
		{
			name: "mixing modules and attributes, ignore attrs",
			input: cfgfile{
				filename: "main.tf",
				body: `
				a = 1
				module "test" {
					source = "test"
				}
				b = 1
			`,
			},
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
				},
			},
		},
		{
			name: "multiple modules",
			input: cfgfile{
				filename: "main.tf",
				body: `
a = 1
module "test" {
	source = "test"
}
b = 1
module "bleh" {
	source = "bleh"
}
`,
			},
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
					{
						Source: "bleh",
					},
				},
			},
		},
		{
			name: "fails if source is not a string",
			input: cfgfile{
				filename: "main.tf",
				body: `
module "test" {
	source = -1
}
`,
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerraformSchema,
					mkrange("main.tf", start(3, 11, 27), end(3, 13, 29)))},
			},
		},
		{
			name: "variable interpolation in the source string - fails",
			input: cfgfile{
				filename: "main.tf",
				body:     "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerraformSchema,
					mkrange("main.tf", start(2, 13, 28), end(2, 16, 31)))},
			},
		},
		{
			name: "multiple schema errors on same file get reported",
			input: cfgfile{
				filename: "main.tf",
				body: `
				module "test" {
					source = -1
				}

				module "test2" {
					source = "${var.test}"
				}

				module {
					source = "test"
				}

				module "test3" {}
			`,
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerraformSchema,
						mkrange("main.tf", start(3, 15, 35), end(3, 17, 37))),
					errors.E(hcl.ErrTerraformSchema,
						mkrange("main.tf", start(7, 18, 83), end(7, 21, 86))),
					errors.E(hcl.ErrTerraformSchema,
						mkrange("main.tf", start(10, 12, 112), end(10, 13, 113))),
					errors.E(hcl.ErrTerraformSchema,
						mkrange("main.tf", start(14, 20, 161), end(14, 22, 163))),
				},
			},
		},
		{
			name: "multiple syntax errors on same file get reported",
			input: cfgfile{
				filename: "main.tf",
				body: `
				string = hi"
				bool   = rue
				list   = [
				obj    = {
			`,
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
					errors.E(mkrange("main.tf", start(2, 17, 17), end(3, 1, 18))),
					errors.E(mkrange("main.tf", start(3, 17, 34), end(4, 1, 35))),
					errors.E(mkrange("main.tf", start(4, 15, 49), end(5, 1, 50))),
					errors.E(mkrange("main.tf", start(5, 15, 64), end(6, 1, 65))),
					errors.E(mkrange("main.tf", start(2, 16, 16), end(2, 17, 17))),
				},
			},
		},
		{
			name: "variable interpolation in the source string - fails",
			input: cfgfile{
				filename: "main.tf",
				body:     "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			},
			want: want{
				errs: []error{errors.E(hcl.ErrTerraformSchema,
					mkrange("main.tf", start(2, 13, 28), end(2, 16, 31)))},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configdir := t.TempDir()
			tfpath := test.WriteFile(t, configdir, tc.input.filename, tc.input.body)
			fixupFiledirOnErrorsFileRanges(configdir, tc.want.errs)

			modules, err := hcl.ParseModules(tfpath)
			errtest.AssertErrorList(t, err, tc.want.errs)
			assert.EqualInts(t,
				len(tc.want.modules),
				len(modules),
				"got: %v, want: %v", modules, tc.want.modules)

			for i := 0; i < len(tc.want.modules); i++ {
				assert.EqualStrings(t, tc.want.modules[i].Source, modules[i].Source,
					"module source mismatch")
			}
		})
	}
}

func TestHCLParserTerramateBlock(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "unrecognized blocks",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     "something {}\nsomething_else {}",
				},
			},
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
				config: hcl.Config{Terramate: &hcl.Terramate{}},
			},
		},
		{
			name: "empty config",
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
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
	for _, tc := range []testcase{
		{
			name: "no config returns empty config",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     `terramate {}`,
				},
			},
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{},
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
			want: want{
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
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
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
									default_branch = "trunk"
									default_remote = "upstream"
									default_branch_base_ref = "HEAD~2"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:        "trunk",
								DefaultRemote:        "upstream",
								DefaultBranchBaseRef: "HEAD~2",
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserStack(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "empty stack block",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "stack with unrecognized blocks",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack{
							block1 {}
							block2 {}
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "multiple stack blocks",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {}
						stack{}
						stack{}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "empty name",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = ""
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "name is not a string - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = 1
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("stack.tm", start(6, 8, 77), end(6, 12, 81)),
					),
				},
			},
		},
		{
			name: "name has interpolation - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = "${test}"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("stack.tm", start(6, 18, 87), end(6, 22, 91))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("stack.tm", start(6, 8, 77), end(6, 12, 81))),
				},
			},
		},
		{
			name: "unrecognized attribute name - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							bleh = "a"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "schema not checked for files with syntax errors",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							wants =
							unrecognized = "test"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
		{
			name: "after: empty set works",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "'after' single entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						After: []string{"test"},
					},
				},
			},
		},
		{
			name: "'after' invalid element entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = [1]
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "'after' invalid type",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = {}
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "'after' null value",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = null
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						After: []string{},
					},
				},
			},
		},
		{
			name: "'after' duplicated entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = ["test", "test"]
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "multiple 'after' fields - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = ["test"]
							after = []
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
		{
			name: "multiple 'before' fields - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							before = []
							before = []
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
		{
			name: "'before' single entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
					},
				},
			},
		},
		{
			name: "'before' multiple entries",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something", "something-else", "test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something", "something-else", "test"},
					},
				},
			},
		},
		{
			name: "stack with valid description",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							description = "some cool description"
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "some cool description",
					},
				},
			},
		},
		{
			name: "stack with multiline description",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
					stack {
						description =  <<-EOD
	line1
	line2
	EOD
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "line1\nline2",
					},
				},
			},
		},
		{
			name: "'before' and 'after'",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
							after = ["else"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
						After:  []string{"else"},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserMultipleErrors(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "multiple syntax errors",
			input: []cfgfile{
				{
					filename: "file.tm",
					body:     "a=1\na=2\na=3",
				},
			},
			want: want{
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
			want: want{
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
			want: want{
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
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						mkrange("cfg1.tm", start(9, 6, 108), end(9, 12, 114))),
					errors.E(hcl.ErrHCLSyntax,
						mkrange("cfg2.tm", start(4, 9, 48), end(4, 23, 62))),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserTerramateBlocksMerging(t *testing.T) {
	tcases := []testcase{
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
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "0.0.1",
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
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
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "6.6.6",
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
							},
						},
					},
					Stack: &hcl.Stack{
						Name:        "stack",
						Description: "some stack",
						After:       []string{"after"},
						Before:      []string{"before"},
						Wants:       []string{"wants"},
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
			want: want{
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
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
	}

	for _, tc := range tcases {
		testParser(t, tc)
	}
}

func testParser(t *testing.T, tc testcase) {
	t.Helper()

	t.Run(tc.name, func(t *testing.T) {
		t.Helper()

		configsDir := t.TempDir()
		for _, inputConfigFile := range tc.input {
			filename := inputConfigFile.filename
			if filename == "" {
				panic("expect a filename in the input config")
			}
			test.WriteFile(t, configsDir, filename, inputConfigFile.body)
		}
		fixupFiledirOnErrorsFileRanges(configsDir, tc.want.errs)
		got, err := hcl.ParseDir(configsDir)
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
		newtc := testcase{
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

// some helpers to easy build file ranges.
func mkrange(fname string, start, end hhcl.Pos) hhcl.Range {
	if start.Byte == end.Byte {
		panic("empty file range")
	}
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

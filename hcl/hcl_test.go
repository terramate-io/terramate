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

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	. "github.com/mineiros-io/terramate/test/hclutils"
	"github.com/mineiros-io/terramate/test/hclutils/info"
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
		name      string
		nonStrict bool
		parsedir  string
		rootdir   string
		input     []cfgfile
		want      want
	}
)

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
						Mkrange("cfg.tm", Start(1, 1, 0), End(1, 10, 9))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(2, 1, 13), End(2, 15, 27))),
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
						Mkrange("cfg.tm", Start(3, 7, 25), End(3, 16, 34))),
				},
			},
		},
		{
			name: "syntax error + unrecognized attribute",
			input: []cfgfile{
				{
					filename: "bug.tm",
					body:     `bug`,
				},
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
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("bug.tm", Start(1, 1, 0), End(1, 4, 3))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(3, 7, 25), End(3, 16, 34))),
				},
			},
		},
		{
			name: "syntax error + invalid import",
			input: []cfgfile{
				{
					filename: "bug.tm",
					body:     `bug`,
				},
				{
					filename: "cfg.tm",
					body: `
						import {
							source = tm_invalid()
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("bug.tm", Start(1, 1, 0), End(1, 4, 3))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(3, 17, 32), End(3, 29, 44))),
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
						Mkrange("cfg.tm", Start(3, 8, 25), End(3, 17, 34))),
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
						Mkrange("cfg.tm", Start(2, 8, 18), End(2, 17, 27))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(3, 8, 38), End(3, 13, 43))),
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
						Mkrange("cfg.tm", Start(3, 27, 45), End(3, 28, 46))),
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
						Mkrange("cfg.tm", Start(3, 30, 48), End(3, 34, 52))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(3, 27, 45), End(3, 44, 62))),
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
						Mkrange("cfg.tm", Start(3, 8, 26), End(3, 15, 33))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(4, 8, 45), End(4, 15, 52))),
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
						Mkrange("cfg.tm", Start(5, 9, 54), End(5, 13, 58))),
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
			want: want{
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
			want: want{
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
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(5, 30, 78), End(5, 34, 82))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(6, 30, 112), End(6, 33, 115))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(7, 30, 145), End(7, 32, 147))),
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
						Mkrange("file.tm", Start(2, 1, 4), End(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("file.tm", Start(3, 1, 8), End(3, 2, 9))),
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
						Mkrange("file1.tm", Start(2, 1, 4), End(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("file1.tm", Start(3, 1, 8), End(3, 2, 9))),
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("file2.tm", Start(2, 1, 4), End(2, 2, 5))),
					errors.E(hcl.ErrHCLSyntax,
						Mkrange("file2.tm", Start(3, 1, 8), End(3, 2, 9))),
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
						Mkrange("stack2.tm", Start(1, 1, 0), End(1, 6, 5))),
				},
			},
		},
		{
			name: "generate_file with lets with child blocks - fails",
			input: []cfgfile{
				{
					filename: "gen.tm",
					body: `
					generate_file "test.tf" {
						lets {
							a = 1
							lets {
								b = 1
							}
						}
						content = ""
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
			name: "generate_hcl with lets with child blocks - fails",
			input: []cfgfile{
				{
					filename: "gen.tm",
					body: `
					generate_hcl "test.tf" {
						lets {
							a = 1
							lets {
								b = 1
							}
						}
						content {
							a = lets.a
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
						Mkrange("cfg1.tm", Start(9, 6, 108), End(9, 10, 112))),
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg2.tm", Start(4, 9, 48), End(4, 23, 62))),
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
			name:      "three config files with terramate and stack blocks",
			nonStrict: true,
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
							wanted_by = ["wanted"]
							watch = ["watch"]
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
						WantedBy:    []string{"wanted"},
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
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name:     "terramate in non-root directory fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/stack.tm",
					body: `
						stack {
							name = "stack"
						}
					`,
				},
				{
					filename: "stack/terramate.tm",
					body: `
						terramate {

						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrUnexpectedTerramate),
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
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
	}

	for _, tc := range tcases {
		testParser(t, tc)
	}
}

func testParser(t *testing.T, tc testcase) {
	t.Run(tc.name, func(t *testing.T) {
		configsDir := t.TempDir()
		for _, inputConfigFile := range tc.input {
			if inputConfigFile.filename == "" {
				panic("expect a filename in the input config")
			}
			path := filepath.Join(configsDir, inputConfigFile.filename)
			dir := filepath.Dir(path)
			filename := filepath.Base(path)
			test.WriteFile(t, dir, filename, inputConfigFile.body)
		}
		FixupFiledirOnErrorsFileRanges(configsDir, tc.want.errs)
		info.FixRangesOnConfig(configsDir, tc.want.config)

		if tc.parsedir == "" {
			tc.parsedir = configsDir
		} else {
			tc.parsedir = filepath.Join(configsDir, tc.parsedir)
		}

		if tc.rootdir == "" {
			tc.rootdir = configsDir
		} else {
			tc.rootdir = filepath.Join(configsDir, tc.rootdir)
		}

		got, err := parse(t, tc)
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

func parse(t *testing.T, tc testcase) (hcl.Config, error) {
	var (
		parser *hcl.TerramateParser
		err    error
	)

	if tc.nonStrict {
		parser, err = hcl.NewTerramateParser(tc.rootdir, tc.parsedir)
	} else {
		parser, err = hcl.NewStrictTerramateParser(tc.rootdir, tc.parsedir)
	}

	if err != nil {
		return hcl.Config{}, err
	}

	err = parser.AddDir(tc.parsedir)
	if err != nil {
		return hcl.Config{}, errors.E("adding files to parser", err)
	}
	return parser.ParseConfig()
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

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

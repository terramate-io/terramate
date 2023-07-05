// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tf_test

import (
	"fmt"
	"path/filepath"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tf"
)

func TestHCLParserModules(t *testing.T) {
	type (
		want struct {
			modules []tf.Module
			errs    []error
		}

		cfgfile struct {
			filename string
			body     string
		}

		testcase struct {
			name  string
			input cfgfile
			want  want
		}
	)

	for _, tc := range []testcase{
		{
			name: "module without label is ignored",
			input: cfgfile{
				filename: "main.tf",
				body:     `module {}`,
			},
		},
		{
			name: "module with N labels is ignored",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "label" "again" {}`,
			},
		},
		{
			name: "module without source attribute is ignored",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "test" {}`,
			},
		},
		{
			name: "empty source is a valid module",
			input: cfgfile{
				filename: "main.tf",
				body:     `module "test" {source = ""}`,
			},
			want: want{
				modules: []tf.Module{
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
				modules: []tf.Module{
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
				modules: []tf.Module{
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
				modules: []tf.Module{
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
			name: "ignored if source is not a string",
			input: cfgfile{
				filename: "main.tf",
				body: `
module "test" {
	source = -1
}
`,
			},
		},
		{
			name: "ignore if there is variable interpolation in the source string",
			input: cfgfile{
				filename: "main.tf",
				body:     "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			},
		},
		{
			name: "multiple ignored module definitions",
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
					errors.E(tf.ErrHCLSyntax),
					errors.E(mkrange("main.tf", start(2, 17, 17), end(3, 1, 18))),
					errors.E(mkrange("main.tf", start(3, 17, 34), end(4, 1, 35))),
					errors.E(mkrange("main.tf", start(4, 15, 49), end(5, 1, 50))),
					errors.E(mkrange("main.tf", start(5, 15, 64), end(6, 1, 65))),
					errors.E(mkrange("main.tf", start(2, 16, 16), end(2, 17, 17))),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configdir := t.TempDir()
			tfpath := test.WriteFile(t, configdir, tc.input.filename, tc.input.body)
			fixupFiledirOnErrorsFileRanges(configdir, tc.want.errs)

			modules, err := tf.ParseModules(tfpath)
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

func TestTerraformHasBackend(t *testing.T) {
	t.Parallel()

	type (
		want struct {
			isStack bool
			err     error
		}

		testcase struct {
			name   string
			layout []string
			want   want
		}
	)

	const filename = "main.tf"

	tfFileLayout := func(content string) string {
		// construct sandbox file builder syntax for creating a
		// single file.
		return fmt.Sprintf(`f:%s:%s`, filename, content)
	}

	for _, tc := range []testcase{
		{
			name: "no blocks defined",
			layout: []string{
				tfFileLayout("# empty content\n"),
			},
			want: want{
				isStack: false,
			},
		},
		{
			name: "terraform block with no backend",
			layout: []string{
				tfFileLayout(`terraform {}`),
			},
			want: want{
				isStack: false,
			},
		},
		{
			name: "terraform block with unrecognized blocks",
			layout: []string{
				tfFileLayout(`terraform {
					abc {}
					xyz {
						a = 1
					}
				}`),
			},
			want: want{
				isStack: false,
			},
		},
		{
			name: "terraform block with a single backend",
			layout: []string{
				tfFileLayout(`terraform {
					abc "label" {}
					xyz {
						a = 1
					}

					backend {
						a = 1
					}

					xpto {
						c = 1
					}
				}`),
			},
			want: want{
				isStack: true,
			},
		},
		{
			name: "terraform block with multiple backends",
			layout: []string{
				tfFileLayout(`terraform {
					backend "a" {
						a = 1
					}
					backend "b" {
						a = 1
					}
					backend "c" {
						a = 1
					}
				}`),
			},
			want: want{
				isStack: true,
			},
		},
		{
			name: "provider block defined",
			layout: []string{
				tfFileLayout(`terraform {
					
				}
				
				provider "aws" {
					attr = 1	
				}`),
			},
			want: want{
				isStack: true,
			},
		},
		{
			name: "multiple provider blocks defined",
			layout: []string{
				tfFileLayout(`				
				provider "aws" {
					attr = 1	
				}
				
				provider "google" {
					attr = 1	
				}
				`),
			},
			want: want{
				isStack: true,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			path := filepath.Join(s.RootDir(), filename)
			hasBackend, err := tf.IsStack(path)
			errtest.Assert(t, err, tc.want.err)

			if err != nil {
				return
			}

			if hasBackend != tc.want.isStack {
				t.Fatalf("unexpected hasBackend. Expected %t but got %t",
					tc.want.isStack, hasBackend)
			}
		})
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

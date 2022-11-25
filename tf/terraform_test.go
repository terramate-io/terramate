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

package tf_test

import (
	"path/filepath"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog"
)

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

func TestHCLParserModules(t *testing.T) {
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

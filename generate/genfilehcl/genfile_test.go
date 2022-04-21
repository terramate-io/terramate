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

package genfilehcl_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate/genfilehcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGeneratedHCL(t *testing.T) {
	type (
		hclconfig struct {
			path     string
			filename string
			add      fmt.Stringer
		}
		genFile struct {
			body   fmt.Stringer
			origin string
		}
		result struct {
			name string
			hcl  genFile
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    []result
			wantErr error
		}
	)

	// attr := func(name, expr string) hclwrite.BlockBuilder {
	// 	t.Helper()
	// 	return hclwrite.AttributeValue(t, name, expr)
	// }
	defaultCfg := func(dir string) string {
		return filepath.Join(dir, config.DefaultFilename)
	}

	tcases := []testcase{
		{
			name:  "generate file with only text in it",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("test.txt"),
						str("content", "hello world"),
					),
				},
			},
			want: []result{
				{
					name: "test.txt",
					hcl: genFile{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							str("content", "hello world"),
						),
					},
				},
			},
		},
		{
			name:  "generate file with only text in it",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("test.txt"),
						str("content", "something = \"something\""),
					),
				},
			},
			want: []result{
				{
					name: "test.txt",
					hcl: genFile{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							str("content", "something = \"something\""),
						),
					},
				},
			},
		},
		{
			name:  "generate file with line break",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("test.txt"),
						str("content", "something = \"something\"\nsomething = \"something\""),
					),
				},
			},
			want: []result{
				{
					name: "test.txt",
					hcl: genFile{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							str("content", "something = \"something\"\nsomething = \"something\""),
						),
					},
				},
			},
		},
		{
			name:  "generate file with function",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("test.json"),
						expr("content", "jsonencode({hello = \"world\"})"),
					),
				},
			},
			want: []result{
				{
					name: "test.json",
					hcl: genFile{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							expr("content", "jsonencode({hello = \"world\"})"),
						),
					},
				},
			},
		},
		{
			name:  "generate file in stack with multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "test.tm.hcl",
					add: generateHCL(
						labels("test.txt"),
						expr("content", "global.some_string"),
					),
				},
				{
					path:     "/stack",
					filename: "test2.tm.hcl",
					add: generateHCL(
						labels("test2.txt"),
						expr("content", "global.some_number"),
					),
				},
				{
					path:     "/stack",
					filename: "test3.tm.hcl",
					add: generateHCL(
						labels("test3.txt"),
						expr("content", "global.some_bool"),
					),
				},
			},
			want: []result{
				{
					name: "test.txt",
					hcl: genFile{
						origin: "/stack/test.tm.hcl",
						body: hcldoc(
							str("content", "string"),
						),
					},
				},
				{
					name: "test2.txt",
					hcl: genFile{
						origin: "/stack/test2.tm.hcl",
						body: hcldoc(
							number("content", 777),
						),
					},
				},
				{
					name: "test3.txt",
					hcl: genFile{
						origin: "/stack/test3.tm.hcl",
						body: hcldoc(
							boolean("content", true),
						),
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				filename := cfg.filename
				if filename == "" {
					filename = config.DefaultFilename
				}
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, filename, cfg.add.String())
			}

			meta := stack.Meta()
			globals := s.LoadStackGlobals(meta)
			res, err := genfilehcl.Load(s.RootDir(), meta, globals)
			assert.IsError(t, err, tcase.wantErr)

			got := res.GeneratedHCLs()

			for _, res := range tcase.want {
				gothcl, ok := got[res.name]
				if !ok {
					t.Fatalf("want hcl code to be generated for %q but no code was generated for it", res.name)
				}
				gotcode := gothcl.String()
				wantcode := res.hcl.body.String()

				assertHCLEquals(t, gotcode, wantcode)
				assert.EqualStrings(t,
					res.hcl.origin,
					gothcl.Origin(),
					"wrong origin config path for generated code",
				)

				delete(got, res.name)
			}

			assert.EqualInts(t, 0, len(got), "got unexpected exported code: %v", got)
		})
	}
}

func TestPartialEval(t *testing.T) {
	// These tests simplify the overall setup/description and focus only
	// on how code will be partially evaluated.
	// No support for multiple config files or generating multiple
	// configurations.
	type testcase struct {
		name    string
		config  hclwrite.BlockBuilder
		globals hclwrite.BlockBuilder
		want    fmt.Stringer
		wantErr error
		skip    bool
	}

	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}

	tcases := []testcase{
		{
			name: "unknown references on count attributes",
			config: hcldoc(
				expr("content", "count.index"),
			),
			want: hcldoc(
				expr("content", "count.index"),
			),
		},
		{
			name: "unknown references on data attributes",
			config: hcldoc(
				expr("content", "data.ref"),
			),
			want: hcldoc(
				expr("content", "data.ref"),
			),
		},
		{
			name: "unknown references on local attributes",
			config: hcldoc(
				expr("content", "local.ref"),
			),
			want: hcldoc(
				expr("content", "local.ref"),
			),
		},
		{
			name: "unknown references on module attributes",
			config: hcldoc(
				expr("content", "module.ref"),
			),
			want: hcldoc(
				expr("content", "module.ref"),
			),
		},
		{
			name: "unknown references on path attributes",
			config: hcldoc(
				expr("content", "path.ref"),
			),
			want: hcldoc(
				expr("content", "path.ref"),
			),
		},
		{
			name: "unknown references on resource attributes",
			config: hcldoc(
				expr("content", "resource.name.etc"),
			),
			want: hcldoc(
				expr("content", "resource.name.etc"),
			),
		},
		{
			name: "unknown references on terraform attributes",
			config: hcldoc(
				expr("content", "terraform.ref"),
			),
			want: hcldoc(
				expr("content", "terraform.ref"),
			),
		},
		{
			name: "mixed references on list",
			globals: hcldoc(
				globals(
					number("ref", 666),
				),
			),
			config: hcldoc(
				expr("content", `[ local.ref, global.ref ]`),
			),
			want: hcldoc(
				expr("content", `[ local.ref, 666 ]`),
			),
		},
		{
			name: "try with unknown reference on attribute is not evaluated",
			config: hcldoc(
				expr("content", "try(something.val, null)"),
			),
			want: hcldoc(
				expr("content", "try(something.val, null)"),
			),
		},
		{
			name: "try with unknown reference on list is not evaluated",
			config: hcldoc(
				expr("content", "[try(something.val, null), 1]"),
			),
			want: hcldoc(
				expr("content", "[try(something.val, null), 1]"),
			),
		},
		{
			name: "function call on attr with mixed references is partially evaluated - global",
			globals: hcldoc(
				globals(
					attr("list", "[1, 2, 3]"),
				),
			),
			config: hcldoc(
				expr("content", "merge(something.val, global.list)"),
			),
			want: hcldoc(
				expr("content", "merge(something.val, [1, 2, 3])"),
			),
		},
		{
			name: "function call on attr with mixed references is partially evaluated - local",
			globals: hcldoc(
				globals(
					attr("list", "[1, 2, 3]"),
				),
			),
			config: hcldoc(
				expr("content", "merge(global.list, local.list)"),
			),
			want: hcldoc(
				expr("content", "merge([1, 2, 3], local.list)"),
			),
		},
		{
			name: "variable interpolation of number with prefix str",
			globals: hcldoc(
				globals(
					number("num", 1337),
				),
			),
			config: hcldoc(
				str("content", `test-${global.num}`),
			),
			want: hcldoc(
				str("content", "test-1337"),
			),
		},
		{
			name: "variable interpolation of bool with prefixed str",
			globals: hcldoc(
				globals(
					boolean("flag", true),
				),
			),
			config: hcldoc(
				str("content", `test-${global.flag}`),
			),
			want: hcldoc(
				str("content", "test-true"),
			),
		},
		{
			name: "variable interpolation without prefixed string",
			globals: hcldoc(
				globals(
					str("string", "hello"),
				),
			),
			config: hcldoc(
				str("content", `${global.string}`),
			),
			want: hcldoc(
				str("content", "hello"),
			),
		},
		{
			name: "variable interpolation with prefixed string",
			globals: hcldoc(
				globals(
					str("string", "hello"),
				),
			),
			config: hcldoc(
				str("content", `test-${global.string}`),
			),
			want: hcldoc(
				str("content", "test-hello"),
			),
		},
		{
			name: "variable interpolation with suffixed string",
			globals: hcldoc(
				globals(
					str("string", "hello"),
				),
			),
			config: hcldoc(
				str("content", `${global.string}-test`),
			),
			want: hcldoc(
				str("content", "hello-test"),
			),
		},
		{
			name: "multiple variable interpolation with prefixed string",
			globals: globals(
				str("string1", "hello1"),
				str("string2", "hello2"),
			),
			config: hcldoc(
				str("content", `something ${global.string1} and ${global.string2}`),
			),
			want: hcldoc(
				str("content", "something hello1 and hello2"),
			),
		},
		{
			name: "multiple variable interpolation without prefixed string",
			globals: hcldoc(
				globals(
					str("string1", "hello1"),
					str("string2", "hello2"),
				),
			),
			config: hcldoc(
				str("content", `${global.string1}${global.string2}`),
			),
			want: hcldoc(
				str("content", "hello1hello2"),
			),
		},
		{
			name: "interpolating multiple objects fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("content", "${global.obj}-${global.obj}"),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "interpolating object with prefix space fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("content", " ${global.obj}"),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "interpolating object with suffix space fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("content", "${global.obj} "),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "interpolating multiple lists fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("content", "${global.list}-${global.list}"),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "interpolating list with prefix space fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("content", " ${global.list}"),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "interpolating list with suffix space fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("content", "${global.list} "),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			// Here we check that a interpolated lists results on the list itself, not a string.
			name: "list interpolation/serialization",
			globals: globals(
				expr("list", `["hi"]`),
			),
			config: hcldoc(
				str("content", "${global.list}"),
			),
			want: hcldoc(
				expr("content", `["hi"]`),
			),
		},
		{
			// Here we check that a interpolated number results on the number itself, not a string.
			name: "number interpolation/serialization",
			globals: globals(
				number("number", 666),
			),
			config: hcldoc(
				str("content", "${global.number}"),
			),
			want: hcldoc(
				number("content", 666),
			),
		},
		{
			// Here we check that multiple interpolated numbers results on a string.
			name: "multiple numbers interpolation/serialization",
			globals: globals(
				number("number", 666),
			),
			config: hcldoc(
				str("content", "${global.number}-${global.number}"),
			),
			want: hcldoc(
				str("content", "666-666"),
			),
		},
		{
			// Here we check that a interpolated booleans results on the boolean itself, not a string.
			name: "boolean interpolation/serialization",
			globals: globals(
				boolean("bool", true),
			),
			config: hcldoc(
				str("content", "${global.bool}"),
			),
			want: hcldoc(
				boolean("content", true),
			),
		},
		{
			// Here we check that multiple interpolated booleans results on a string.
			name: "multiple booleans interpolation/serialization",
			globals: globals(
				boolean("bool", true),
			),
			config: hcldoc(
				str("content", "${global.bool}-${global.bool}"),
			),
			want: hcldoc(
				str("content", "true-true"),
			),
		},
		{
			name: "test list - just to see how hcl lib serializes a list // remove me",
			globals: hcldoc(
				globals(
					expr("list", `[1, 2, 3]`),
					str("interp", "${global.list}"),
				),
			),
			config: hcldoc(
				str("content", "${global.interp}"),
			),
			want: hcldoc(
				expr("content", "[1, 2, 3]"),
			),
		},
		{
			name: "variable list interpolation/serialization in a string",
			globals: hcldoc(
				globals(
					expr("list", `[1, 2, 3]`),
				),
			),
			config: hcldoc(
				str("content", "${global.list}"),
			),
			want: hcldoc(
				expr("content", "[1, 2, 3]"),
			),
		},
		{
			name: "basic plus expression",
			config: hcldoc(
				expr("content", `1 + 1`),
			),
			want: hcldoc(
				expr("content", `1 + 1`),
			),
		},
		{
			name: "plus expression funcall",
			config: hcldoc(
				expr("content", `len(a.b) + len2(c.d)`),
			),
			want: hcldoc(
				expr("content", `len(a.b) + len2(c.d)`),
			),
		},
		{
			name: "plus expression evaluated",
			globals: hcldoc(
				globals(
					str("a", "hello"),
					str("b", "world"),
				),
			),
			config: hcldoc(
				expr("content", `tm_upper(global.a) + tm_upper(global.b)`),
			),
			want: hcldoc(
				expr("content", `"HELLO" + "WORLD"`),
			),
		},
		{
			name: "plus expression evaluated advanced",
			globals: hcldoc(
				globals(
					str("a", "hello"),
					str("b", "world"),
				),
			),
			config: hcldoc(
				expr("content", `tm_lower(tm_upper(global.a)) + tm_lower(tm_upper(global.b))`),
			),
			want: hcldoc(
				expr("content", `"hello" + "world"`),
			),
		},
		{
			name: "basic minus expression",
			config: hcldoc(
				expr("content", `1 + 1`),
			),
			want: hcldoc(
				expr("content", `1 + 1`),
			),
		},
		{
			name: "conditional expression",
			config: hcldoc(
				expr("content", `1 == 1 ? 0 : 1`),
			),
			want: hcldoc(
				expr("content", `1 == 1 ? 0 : 1`),
			),
		},
		{
			name: "conditional expression 2",
			globals: hcldoc(
				globals(
					number("num", 10),
				),
			),
			config: hcldoc(
				expr("content", `1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: hcldoc(
				expr("content", `1 >= 10 ? local.x : [for x in local.a : x]`),
			),
		},
		{
			name: "operation + conditional expression",
			globals: hcldoc(
				globals(
					number("num", 10),
				),
			),
			config: hcldoc(
				expr("content", `local.x + 1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: hcldoc(
				expr("content", `local.x + 1 >= 10 ? local.x : [for x in local.a : x]`),
			),
		},
		{
			name: "deep object interpolation",
			globals: hcldoc(
				globals(
					expr("obj", `{
						obj2 = {
							obj3 = {
								string = "hello"
								number = 1337
								bool = false
							}
						}
						string = "hello"
						number = 1337
						bool = false
					}`),
				),
			),
			config: hcldoc(
				str("content", "${global.obj.string} ${global.obj.obj2.obj3.number}"),
			),
			want: hcldoc(
				str("content", "hello 1337"),
			),
		},
		{
			name: "deep object interpolation of object field and str field fails",
			globals: hcldoc(
				globals(
					expr("obj", `{
						obj2 = {
							obj3 = {
								string = "hello"
								number = 1337
								bool = false
							}
						}
						string = "hello"
						number = 1337
						bool = false
					}`),
				),
			),
			config: hcldoc(
				str("content", "${global.obj.string} ${global.obj.obj2.obj3}"),
			),
			wantErr: eval.ErrInterpolationEval,
		},
		{
			name: "basic list indexing",
			globals: hcldoc(
				globals(
					expr("list", `["a", "b", "c"]`),
				),
			),
			config: hcldoc(
				expr("content", `global.list[0]`),
			),
			want: hcldoc(
				str("content", "a"),
			),
		},
		{
			name: "advanced list indexing",
			skip: true,
			globals: hcldoc(
				globals(
					expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: hcldoc(
				expr("content", `global.list[1][1]`),
			),
			want: hcldoc(
				number("content", 5),
			),
		},
		{
			name: "advanced list indexing 2",
			skip: true,
			globals: hcldoc(
				globals(
					expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: hcldoc(
				expr("content", `global.list[1+1][1-1]`),
			),
			want: hcldoc(
				number("content", 7),
			),
		},
		{
			name: "advanced object indexing",
			skip: true,
			globals: hcldoc(
				globals(
					expr("obj", `{A = {B = "test"}}`),
				),
			),
			config: hcldoc(
				expr("content", `global.list[tm_upper("a")][tm_upper("b)]`),
			),
			want: hcldoc(
				str("content", "test"),
			),
		},
		{
			name: "basic object indexing",
			globals: hcldoc(
				globals(
					expr("obj", `{"a" = "b"}`),
				),
			),
			config: hcldoc(
				expr("content", `global.obj["a"]`),
			),
			want: hcldoc(
				str("content", "b"),
			),
		},
		{
			name: "obj for loop without eval references",
			config: hcldoc(
				expr("content", `{for k in local.list : k => k}`),
			),
			want: hcldoc(
				expr("content", `{for k in local.list : k => k}`),
			),
		},
		{
			name: "list for loop without eval references",
			config: hcldoc(
				expr("content", `[for k in local.list : k]`),
			),
			want: hcldoc(
				expr("content", `[for k in local.list : k]`),
			),
		},
		{
			name: "{for loop from map and funcall",
			config: hcldoc(
				expr("content", `{for s in var.list : s => upper(s)}`),
			),
			want: hcldoc(
				expr("content", `{for s in var.list : s => upper(s)}`),
			),
		},
		{
			name: "{for in from {for map",
			config: hcldoc(
				expr("content", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
			want: hcldoc(
				expr("content", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
		},
		{
			name: "[for with funcall",
			config: hcldoc(
				expr("content", `[for s in var.list : upper(s)]`),
			),
			want: hcldoc(
				expr("content", `[for s in var.list : upper(s)]`),
			),
		},
		{
			name: "[for in from map and Operation body",
			config: hcldoc(
				expr("content", `[for k, v in var.map : length(k) + length(v)]`),
			),
			want: hcldoc(
				expr("content", `[for k, v in var.map : length(k) + length(v)]`),
			),
		},
		{
			name: "[for in from map and interpolation body",
			config: hcldoc(
				expr("content", `[for i, v in var.list : "${i} is ${v}"]`),
			),
			want: hcldoc(
				expr("content", `[for i, v in var.list : "${i} is ${v}"]`),
			),
		},
		{
			name: "[for in from map with conditional body",
			config: hcldoc(
				expr("content", `[for s in var.list : upper(s) if s != ""]`),
			),
			want: hcldoc(
				expr("content", `[for s in var.list : upper(s) if s != ""]`),
			),
		},
		{
			name: "[for in from [for list",
			config: hcldoc(
				expr("content", `[for s in [for s in a.b : s] : s]`),
			),
			want: hcldoc(
				expr("content", `[for s in [for s in a.b : s] : s]`),
			),
		},
		{
			name: "list for loop with global reference fails",
			globals: globals(
				expr("list", `["a", "b", "c"]`),
			),
			config: hcldoc(
				expr("content", `[for k in global.list : k]`),
			),
			wantErr: eval.ErrForExprDisallowEval,
		},
		{
			name: "obj for loop with global reference fails",
			globals: globals(
				expr("obj", `{ a = 1}`),
			),
			config: hcldoc(
				expr("content", `[for k in global.obj : k]`),
			),
			wantErr: eval.ErrForExprDisallowEval,
		},
		{
			name: "[for in from [for list with global references",
			globals: globals(
				expr("list", `["a", "b", "c"]`),
			),
			config: hcldoc(
				expr("content", `[for s in [for s in global.list : s] : s]`),
			),
			wantErr: eval.ErrForExprDisallowEval,
		},
		{
			name: "mixing {for and [for",
			config: hcldoc(
				expr("content", `{for k, v in [for k in a.b : k] : k => v}`),
			),
			want: hcldoc(
				expr("content", `{for k, v in [for k in a.b : k] : k => v}`),
			),
		},
		{
			name: "unary operation !",
			config: hcldoc(
				expr("content", "!0"),
			),
			want: hcldoc(
				expr("content", "!0"),
			),
		},
		{
			name: "unary operation -",
			config: hcldoc(
				expr("content", "-0"),
			),
			want: hcldoc(
				expr("content", "-0"),
			),
		},
		{
			name: "number indexing",
			config: hcldoc(
				expr("content", "b.1000"),
			),
			want: hcldoc(
				expr("content", "b.1000"),
			),
		},
		{
			name: "advanced number literal",
			config: hcldoc(
				expr("content", "10.1200"),
			),
			want: hcldoc(
				expr("content", "10.1200"),
			),
		},
		{
			name: "advanced number literal",
			config: hcldoc(
				expr("content", "0.0.A.0"),
			),
			want: hcldoc(
				expr("content", "0.0.A.0"),
			),
		},
		{
			name: "parenthesis and splat with newlines",
			config: hcldoc(
				expr("content", "(A(). \n*)"),
			),
			want: hcldoc(
				expr("content", "(A(). \n*)"),
			),
		},
		{
			name: "funcall and newlines/comments",
			config: hcldoc(
				expr("content", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
			want: hcldoc(
				expr("content", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
		},
		{
			name: "tm_ funcall and newlines/comments",
			config: hcldoc(
				expr("content", "tm_try(\n/**/a\n/**/,/**/b, null/**/\n/**/)"),
			),
			want: hcldoc(
				expr("content", "null"),
			),
		},
		{
			name: "objects and newlines/comments",
			config: hcldoc(
				expr("content", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
			want: hcldoc(
				expr("content", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
		},
		{
			name: "lists and newlines/comments",
			config: hcldoc(
				expr("content", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
			want: hcldoc(
				expr("content", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
		},
		{
			name: "conditional globals evaluation",
			globals: globals(
				str("domain", "mineiros.io"),
				boolean("exists", true),
			),
			config: hcldoc(
				expr("content", `global.exists ? global.domain : "example.com"`),
			),
			want: hcldoc(
				expr("content", `true ? "mineiros.io" : "example.com"`),
			),
		},
		{
			name: "evaluated empty string in the prefix",
			config: hcldoc(
				expr("content", "\"${tm_replace(0,\"0\",\"\")}0\""),
			),
			want: hcldoc(
				expr("content", "\"0\""),
			),
		},
		{
			name: "evaluated empty string in the suffix",
			config: hcldoc(
				expr("content", "\"0${tm_replace(0,\"0\",\"\")}\""),
			),
			want: hcldoc(
				expr("content", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines prefix",
			config: hcldoc(
				expr("content", "\"${\ntm_replace(0,0,\"\")}0\""),
			),
			want: hcldoc(
				expr("content", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines suffix",
			config: hcldoc(
				expr("content", "\"${tm_replace(0,0,\"\")\n}0\""),
			),
			want: hcldoc(
				expr("content", "\"0\""),
			),
		},
		{
			name: "lists and newlines/comments",
			config: hcldoc(
				expr("content", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
			want: hcldoc(
				expr("content", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
		},
		{
			name: "interpolation advanced 1",
			globals: globals(
				str("a", "1"),
			),
			config: hcldoc(
				str("content", "0${tm_try(global.a)}2"),
			),
			want: hcldoc(
				str("content", "012"),
			),
		},
		{
			name: "escaped interpolation with global reference",
			config: hcldoc(
				str("content", `$${global.string}`),
			),
			want: hcldoc(
				str("content", "$${global.string}"),
			),
		},
		{
			name: "escaped interpolation with attr",
			config: hcldoc(
				str("content", `$${hi}`),
			),
			want: hcldoc(
				str("content", "$${hi}"),
			),
		},
		{
			name: "escaped interpolation with number",
			config: hcldoc(
				str("content", `$${5}`),
			),
			want: hcldoc(
				str("content", "$${5}"),
			),
		},
		{
			name: "empty escaped interpolation",
			config: hcldoc(
				str("content", `$${}`),
			),
			want: hcldoc(
				str("content", "$${}"),
			),
		},
		{
			name: "escaped interpolation with prefix",
			config: hcldoc(
				str("content", `something-$${hi}`),
			),
			want: hcldoc(
				str("content", "something-$${hi}"),
			),
		},
		{
			name: "escaped interpolation with suffix",
			config: hcldoc(
				str("content", `$${hi}-suffix`),
			),
			want: hcldoc(
				str("content", "$${hi}-suffix"),
			),
		},
		{
			name: "nested escaped interpolation",
			config: hcldoc(
				str("content", `$${hi$${again}}`),
			),
			want: hcldoc(
				str("content", `$${hi$${again}}`),
			),
		},
		{
			name: "interpolation inside escaped interpolation",
			config: hcldoc(
				str("content", `$${hi${attr}}`),
			),
			want: hcldoc(
				str("content", `$${hi${attr}}`),
			),
		},
		{
			name: "global interpolation inside escaped interpolation",
			globals: globals(
				number("a", 666),
			),
			config: hcldoc(
				str("content", `$${hi-${global.a}}`),
			),
			want: hcldoc(
				str("content", `$${hi-666}`),
			),
		},
		{
			name: "for inside escaped interpolation",
			config: hcldoc(
				str("content", `$${[for k in local.a : k]}`),
			),
			want: hcldoc(
				str("content", `$${[for k in local.a : k]}`),
			),
		},
		{
			name: "for inside escaped interpolation referencing global",
			config: hcldoc(
				str("content", `$${[for k in global.a : k]}`),
			),
			want: hcldoc(
				str("content", `$${[for k in global.a : k]}`),
			),
		},
		/*
			 * Hashicorp HCL formats the `wants` wrong.
			 *
			{
				name: "interpolation advanced 2",
				globals: globals(
					str("a", "1"),
				),
				config: hcldoc(
					str("a", "0${!tm_try(global.a)}2"),
				),
				want: hcldoc(
					str("a", `0${!"1"}2`),
				),
			},
		*/
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			const (
				stackname = "stack"
				genname   = "test"
			)

			if tcase.skip {
				t.Skip()
			}

			s := sandbox.New(t)
			stackEntry := s.CreateStack(stackname)
			stack := stackEntry.Load()
			path := filepath.Join(s.RootDir(), stackname)
			if tcase.globals == nil {
				tcase.globals = globals()
			}
			cfg := hcldoc(
				tcase.globals,
				generateHCL(
					labels(genname),
					tcase.config,
				),
			)

			t.Logf("input: %s", cfg.String())
			test.AppendFile(t, path, config.DefaultFilename, cfg.String())

			meta := stack.Meta()
			globals := s.LoadStackGlobals(meta)
			res, err := genfilehcl.Load(s.RootDir(), meta, globals)
			assert.IsError(t, err, tcase.wantErr)

			if err != nil {
				return
			}

			got := res.GeneratedHCLs()

			assert.EqualInts(t, len(got), 1, "want single generated HCL")

			gothcl := got[genname]
			gotcode := gothcl.String()
			wantcode := tcase.want.String()
			assertHCLEquals(t, gotcode, wantcode)
		})
	}
}

func assertHCLEquals(t *testing.T, got string, want string) {
	t.Helper()

	const trimmedChars = "\n "

	got = strings.Trim(got, trimmedChars)
	want = strings.Trim(want, trimmedChars)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("generated code doesn't match expectation")
		t.Errorf("want:\n%q", want)
		t.Errorf("got:\n%q", got)
		t.Fatalf("diff:\n%s", diff)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

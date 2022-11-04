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

package genhcl_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestPartialEval(t *testing.T) {
	t.Parallel()
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
	}

	hugestr := strings.Repeat("huge ", 1000)
	tcases := []testcase{
		{
			name: "unknown references on attributes",
			config: Doc(
				Expr("count", "count.index"),
				Expr("data", "data.ref"),
				Expr("local", "local.ref"),
				Expr("module", "module.ref"),
				Expr("path", "path.ref"),
				Expr("resource", "resource.name.etc"),
				Expr("terraform", "terraform.ref"),
			),
			want: Doc(
				Expr("count", "count.index"),
				Expr("data", "data.ref"),
				Expr("local", "local.ref"),
				Expr("module", "module.ref"),
				Expr("path", "path.ref"),
				Expr("resource", "resource.name.etc"),
				Expr("terraform", "terraform.ref"),
			),
		},
		{
			name: "unknown references on object",
			config: Doc(
				Expr("obj", `{
					count     = count.index,
					data      = data.ref,
					local     = local.ref,
					module    = module.ref,
					path      = path.ref,
					resource  = resource.name.etc,
					terraform = terraform.ref,
				 }`),
			),
			want: Doc(
				Expr("obj", `{
					count     = count.index,
					data      = data.ref,
					local     = local.ref,
					module    = module.ref,
					path      = path.ref,
					resource  = resource.name.etc,
					terraform = terraform.ref,
				 }`),
			),
		},
		{
			name: "mixed references on same object",
			globals: Doc(
				Globals(
					Number("ref", 666),
				),
			),
			config: Doc(
				Expr("obj", `{
					local     = local.ref,
					global    = global.ref,
				 }`),
			),
			want: Doc(
				Expr("obj", `{
					local   = local.ref,
					global  = 666,
				 }`),
			),
		},
		{
			name: "mixed references on list",
			globals: Doc(
				Globals(
					Number("ref", 666),
				),
			),
			config: Doc(
				Expr("list", `[ local.ref, global.ref ]`),
			),
			want: Doc(
				Expr("list", `[ local.ref, 666 ]`),
			),
		},
		{
			name: "try with unknown reference on attribute is not evaluated",
			config: Doc(
				Expr("attr", "try(something.val, null)"),
			),
			want: Doc(
				Expr("attr", "try(something.val, null)"),
			),
		},
		{
			name: "try with unknown reference on list is not evaluated",
			config: Doc(
				Expr("list", "[try(something.val, null), 1]"),
			),
			want: Doc(
				Expr("list", "[try(something.val, null), 1]"),
			),
		},
		{
			name: "try with unknown reference on object is not evaluated",
			config: Doc(
				Expr("obj", `{
					a = try(something.val, null),	
					b = "val",
				}`),
			),
			want: Doc(
				Expr("obj", `{
					a = try(something.val, null),	
					b = "val",
				}`),
			),
		},
		{
			name: "variable definition with optionals",
			config: Doc(
				Variable(
					Labels("with_optional_attribute"),
					Expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 1)
					})`),
				),
			),
			want: Doc(
				Variable(
					Labels("with_optional_attribute"),
					Expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 1)
					})`),
				),
			),
		},
		{
			name: "variable definition with optional default from global",
			globals: Doc(
				Globals(
					Number("default", 666),
				),
			),
			config: Doc(
				Variable(
					Labels("with_optional_attribute"),
					Expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, global.default)
					})`),
				),
			),
			want: Doc(
				Variable(
					Labels("with_optional_attribute"),
					Expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 666)
					})`),
				),
			),
		},
		{
			name: "function call on attr with mixed references is partially evaluated",
			globals: Doc(
				Globals(
					EvalExpr(t, "list", "[1, 2, 3]"),
				),
			),
			config: Doc(
				Expr("a", "merge(something.val, global.list)"),
				Expr("b", "merge(global.list, local.list)"),
			),
			want: Doc(
				Expr("a", "merge(something.val, [1, 2, 3])"),
				Expr("b", "merge([1, 2, 3], local.list)"),
			),
		},
		{
			name: "function call on obj with mixed references is partially evaluated",
			globals: Doc(
				Globals(
					EvalExpr(t, "list", "[1, 2, 3]"),
				),
			),
			config: Doc(
				Expr("obj", `{
					a = merge(something.val, global.list)
				}`),
			),
			want: Doc(
				Expr("obj", `{
					a = merge(something.val, [1, 2, 3])
				}`),
			),
		},
		{
			name: "variable interpolation of number with prefix str",
			globals: Doc(
				Globals(
					Number("num", 1337),
				),
			),
			config: Doc(
				Str("num", `test-${global.num}`),
			),
			want: Doc(
				Str("num", "test-1337"),
			),
		},
		{
			name: "variable interpolation of bool with prefixed str",
			globals: Doc(
				Globals(
					Bool("flag", true),
				),
			),
			config: Doc(
				Str("flag", `test-${global.flag}`),
			),
			want: Doc(
				Str("flag", "test-true"),
			),
		},
		{
			name: "variable interpolation without prefixed string",
			globals: Doc(
				Globals(
					Str("string", "hello"),
				),
			),
			config: Doc(
				Str("string", `${global.string}`),
			),
			want: Doc(
				Str("string", "hello"),
			),
		},
		{
			name: "variable interpolation with prefixed string",
			globals: Doc(
				Globals(
					Str("string", "hello"),
				),
			),
			config: Doc(
				Str("string", `test-${global.string}`),
			),
			want: Doc(
				Str("string", "test-hello"),
			),
		},
		{
			name: "variable interpolation with suffixed string",
			globals: Doc(
				Globals(
					Str("string", "hello"),
				),
			),
			config: Doc(
				Str("string", `${global.string}-test`),
			),
			want: Doc(
				Str("string", "hello-test"),
			),
		},
		{
			name: "multiple variable interpolation with prefixed string",
			globals: Globals(
				Str("string1", "hello1"),
				Str("string2", "hello2"),
			),
			config: Doc(
				Str("string", `something ${global.string1} and ${global.string2}`),
			),
			want: Doc(
				Str("string", "something hello1 and hello2"),
			),
		},
		{
			name: "multiple variable interpolation without prefixed string",
			globals: Doc(
				Globals(
					Str("string1", "hello1"),
					Str("string2", "hello2"),
				),
			),
			config: Doc(
				Str("string", `${global.string1}${global.string2}`),
			),
			want: Doc(
				Str("string", "hello1hello2"),
			),
		},
		{
			// Here we check that an interpolated object results on the object itself, not a string.
			name: "object interpolation/serialization",
			globals: Globals(
				Expr("obj", `{
					string = "hello"
					number = 1337
					bool = false
				}`),
			),
			config: Doc(
				Expr("obj", "global.obj"),
				Str("obj_interpolated", "${global.obj}"),
			),
			want: Doc(
				Expr("obj", `{
					bool = false
					number = 1337
					string = "hello"
				}`),
				Expr("obj_interpolated", `{
					bool = false
					number = 1337
					string = "hello"
				}`),
			),
		},
		{
			name: "interpolating multiple objects fails",
			globals: Globals(
				Expr("obj", `{ string = "hello" }`),
			),
			config: Doc(
				Str("a", "${global.obj}-${global.obj}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating object with prefix space fails",
			globals: Globals(
				Expr("obj", `{ string = "hello" }`),
			),
			config: Doc(
				Str("a", " ${global.obj}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating object with suffix space fails",
			globals: Globals(
				Expr("obj", `{ string = "hello" }`),
			),
			config: Doc(
				Str("a", "${global.obj} "),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating multiple lists fails",
			globals: Globals(
				Expr("list", `["hello"]`),
			),
			config: Doc(
				Str("a", "${global.list}-${global.list}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating list with prefix space fails",
			globals: Globals(
				Expr("list", `["hello"]`),
			),
			config: Doc(
				Str("a", " ${global.list}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating list with suffix space fails",
			globals: Globals(
				Expr("list", `["hello"]`),
			),
			config: Doc(
				Str("a", "${global.list} "),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			// Here we check that a interpolated lists results on the list itself, not a string.
			name: "list interpolation/serialization",
			globals: Globals(
				Expr("list", `["hi"]`),
			),
			config: Doc(
				Expr("list", "global.list"),
				Str("list_interpolated", "${global.list}"),
			),
			want: Doc(
				Expr("list", `["hi"]`),
				Expr("list_interpolated", `["hi"]`),
			),
		},
		{
			// Here we check that a interpolated number results on the number itself, not a string.
			name: "number interpolation/serialization",
			globals: Globals(
				Number("number", 666),
			),
			config: Doc(
				Expr("number", "global.number"),
				Str("number_interpolated", "${global.number}"),
			),
			want: Doc(
				Number("number", 666),
				Number("number_interpolated", 666),
			),
		},
		{
			// Here we check that multiple interpolated numbers results on a string.
			name: "multiple numbers interpolation/serialization",
			globals: Globals(
				Number("number", 666),
			),
			config: Doc(
				Expr("number", "global.number"),
				Str("number_interpolated", "${global.number}-${global.number}"),
			),
			want: Doc(
				Number("number", 666),
				Str("number_interpolated", "666-666"),
			),
		},
		{
			// Here we check that a interpolated booleans results on the boolean itself, not a string.
			name: "boolean interpolation/serialization",
			globals: Globals(
				Bool("bool", true),
			),
			config: Doc(
				Expr("bool", "global.bool"),
				Str("bool_interpolated", "${global.bool}"),
			),
			want: Doc(
				Bool("bool", true),
				Bool("bool_interpolated", true),
			),
		},
		{
			// Here we check that multiple interpolated booleans results on a string.
			name: "multiple booleans interpolation/serialization",
			globals: Globals(
				Bool("bool", true),
			),
			config: Doc(
				Expr("bool", "global.bool"),
				Str("bool_interpolated", "${global.bool}-${global.bool}"),
			),
			want: Doc(
				Bool("bool", true),
				Str("bool_interpolated", "true-true"),
			),
		},
		{
			name: "test list - just to see how hcl lib serializes a list // remove me",
			globals: Doc(
				Globals(
					Expr("list", `[1, 2, 3]`),
					Str("interp", "${global.list}"),
				),
			),
			config: Doc(
				Str("var", "${global.interp}"),
			),
			want: Doc(
				Expr("var", "[1, 2, 3]"),
			),
		},
		{
			name: "variable list interpolation/serialization in a string",
			globals: Doc(
				Globals(
					Expr("list", `[1, 2, 3]`),
				),
			),
			config: Doc(
				Str("var", "${global.list}"),
			),
			want: Doc(
				Expr("var", "[1, 2, 3]"),
			),
		},
		{
			name: "basic plus expression",
			config: Doc(
				Expr("var", `1 + 1`),
			),
			want: Doc(
				Expr("var", `1 + 1`),
			),
		},
		{
			name: "plus expression funcall",
			config: Doc(
				Expr("var", `len(a.b) + len2(c.d)`),
			),
			want: Doc(
				Expr("var", `len(a.b) + len2(c.d)`),
			),
		},
		{
			name: "plus expression evaluated",
			globals: Doc(
				Globals(
					Str("a", "hello"),
					Str("b", "world"),
				),
			),
			config: Doc(
				Expr("var", `tm_upper(global.a) + tm_upper(global.b)`),
			),
			want: Doc(
				Expr("var", `"HELLO" + "WORLD"`),
			),
		},
		{
			name: "plus expression evaluated advanced",
			globals: Doc(
				Globals(
					Str("a", "hello"),
					Str("b", "world"),
				),
			),
			config: Doc(
				Expr("var", `tm_lower(tm_upper(global.a)) + tm_lower(tm_upper(global.b))`),
			),
			want: Doc(
				Expr("var", `"hello" + "world"`),
			),
		},
		{
			name: "basic minus expression",
			config: Doc(
				Expr("var", `1 + 1`),
			),
			want: Doc(
				Expr("var", `1 + 1`),
			),
		},
		{
			name: "conditional expression",
			config: Doc(
				Expr("var", `1 == 1 ? 0 : 1`),
			),
			want: Doc(
				Expr("var", `1 == 1 ? 0 : 1`),
			),
		},
		{
			name: "conditional expression 2",
			globals: Doc(
				Globals(
					Number("num", 10),
				),
			),
			config: Doc(
				Expr("var", `1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: Doc(
				Expr("var", `1 >= 10 ? local.x : [for x in local.a : x]`),
			),
		},
		{
			name: "operation + conditional expression",
			globals: Doc(
				Globals(
					Number("num", 10),
				),
			),
			config: Doc(
				Expr("var", `local.x + 1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: Doc(
				Expr("var", `local.x + 1 >= 10 ? local.x : [for x in local.a : x]`),
			),
		},
		{
			name: "deep object interpolation",
			globals: Doc(
				Globals(
					Expr("obj", `{
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
			config: Doc(
				Str("var", "${global.obj.string} ${global.obj.obj2.obj3.number}"),
			),
			want: Doc(
				Str("var", "hello 1337"),
			),
		},
		{
			name: "deep object interpolation of object field and str field fails",
			globals: Doc(
				Globals(
					Expr("obj", `{
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
			config: Doc(
				Str("var", "${global.obj.string} ${global.obj.obj2.obj3}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "basic list indexing",
			globals: Doc(
				Globals(
					Expr("list", `["a", "b", "c"]`),
				),
			),
			config: Doc(
				Expr("string", `global.list[0]`),
			),
			want: Doc(
				Str("string", "a"),
			),
		},
		{
			name: "advanced list indexing",
			globals: Doc(
				Globals(
					Expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: Doc(
				Expr("num", `global.list[1][1]`),
			),
			want: Doc(
				Number("num", 5),
			),
		},
		{
			name: "advanced list indexing 2",
			globals: Doc(
				Globals(
					Expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: Doc(
				Expr("num", `global.list[1+1][1-1]`),
			),
			want: Doc(
				Number("num", 7),
			),
		},
		{
			name: "advanced object indexing",
			globals: Doc(
				Globals(
					Expr("obj", `{A = {B = "test"}}`),
				),
			),
			config: Doc(
				Expr("string", `global.obj[tm_upper("a")][tm_upper("b")]`),
			),
			want: Doc(
				Str("string", "test"),
			),
		},
		{
			name: "basic object indexing",
			globals: Doc(
				Globals(
					Expr("obj", `{"a" = "b"}`),
				),
			),
			config: Doc(
				Expr("string", `global.obj["a"]`),
			),
			want: Doc(
				Str("string", "b"),
			),
		},
		{
			name: "indexing of outside variables",
			globals: Doc(
				Globals(
					Number("depth", 1),
				),
			),
			config: Doc(
				Expr("folder_id", `data.google_active_folder[global.depth].0.id`),
			),
			want: Doc(
				Expr("folder_id", `data.google_active_folder[1].0.id`),
			),
		},
		{
			name: "indexing of outside variables with interpolation of single var",
			globals: Doc(
				Globals(
					Number("depth", 1),
				),
			),
			config: Doc(
				Expr("folder_id", `data.google_active_folder["${global.depth}"].0.id`),
			),
			want: Doc(
				Expr("folder_id", `data.google_active_folder[1].0.id`),
			),
		},
		{
			name: "indexing of outside variables with interpolation",
			globals: Doc(
				Globals(
					Number("depth", 1),
				),
			),
			config: Doc(
				Expr("folder_id", `data.google_active_folder["l${global.depth}"].0.id`),
			),
			want: Doc(
				Expr("folder_id", `data.google_active_folder["l1"].0.id`),
			),
		},
		{
			name: "outside variable with splat operator",
			config: Doc(
				Expr("folder_id", `data.test[*].0.id`),
			),
			want: Doc(
				Expr("folder_id", `data.test[*].0.id`),
			),
		},
		{
			name: "outside variable with splat getattr operator",
			config: Doc(
				Expr("folder_id", `data.test.*.0.id`),
			),
			want: Doc(
				Expr("folder_id", `data.test.*.0.id`),
			),
		},
		{
			name: "multiple indexing",
			config: Doc(
				Expr("a", `data.test[0][0][0]`),
			),
			want: Doc(
				Expr("a", `data.test[0][0][0]`),
			),
		},
		{
			name: "multiple indexing with evaluation",
			globals: Doc(
				Globals(
					Number("val", 1),
				),
			),
			config: Doc(
				Expr("a", `data.test[global.val][0][0]`),
			),
			want: Doc(
				Expr("a", `data.test[1][0][0]`),
			),
		},
		{
			name: "multiple indexing with evaluation 2",
			globals: Doc(
				Globals(
					Number("val", 1),
				),
			),
			config: Doc(
				Expr("a", `data.test[0][global.val][global.val+1]`),
			),
			want: Doc(
				Expr("a", `data.test[0][1][1+1]`),
			),
		},
		{
			name: "nested indexing",
			globals: Doc(
				Globals(
					Expr("obj", `{
						key = {
							key2 = {
								val = "hello"
							}
						}
					}`),
					Expr("obj2", `{
						keyname = "key"
					}`),
					Expr("key", `{
						key2 = "keyname"
					}`),
					Str("key2", "key2"),
				),
			),
			config: Doc(
				Expr("hello", `global.obj[global.obj2[global.key[global.key2]]][global.key2]["val"]`),
			),
			want: Doc(
				Str("hello", "hello"),
			),
		},
		{
			name: "obj for loop without eval references",
			config: Doc(
				Expr("obj", `{for k in local.list : k => k}`),
			),
			want: Doc(
				Expr("obj", `{for k in local.list : k => k}`),
			),
		},
		{
			name: "list for loop without eval references",
			config: Doc(
				Expr("obj", `[for k in local.list : k]`),
			),
			want: Doc(
				Expr("obj", `[for k in local.list : k]`),
			),
		},
		{
			name: "{for loop from map and funcall",
			config: Doc(
				Expr("obj", `{for s in var.list : s => upper(s)}`),
			),
			want: Doc(
				Expr("obj", `{for s in var.list : s => upper(s)}`),
			),
		},
		{
			name: "{for in from {for map",
			config: Doc(
				Expr("obj", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
			want: Doc(
				Expr("obj", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
		},
		{
			name: "[for with funcall",
			config: Doc(
				Expr("obj", `[for s in var.list : upper(s)]`),
			),
			want: Doc(
				Expr("obj", `[for s in var.list : upper(s)]`),
			),
		},
		{
			name: "[for in from map and Operation body",
			config: Doc(
				Expr("obj", `[for k, v in var.map : length(k) + length(v)]`),
			),
			want: Doc(
				Expr("obj", `[for k, v in var.map : length(k) + length(v)]`),
			),
		},
		{
			name: "[for in from map and interpolation body",
			config: Doc(
				Expr("obj", `[for i, v in var.list : "${i} is ${v}"]`),
			),
			want: Doc(
				Expr("obj", `[for i, v in var.list : "${i} is ${v}"]`),
			),
		},
		{
			name: "[for in from map with conditional body",
			config: Doc(
				Expr("obj", `[for s in var.list : upper(s) if s != ""]`),
			),
			want: Doc(
				Expr("obj", `[for s in var.list : upper(s) if s != ""]`),
			),
		},
		{
			name: "[for in from [for list",
			config: Doc(
				Expr("obj", `[for s in [for s in a.b : s] : s]`),
			),
			want: Doc(
				Expr("obj", `[for s in [for s in a.b : s] : s]`),
			),
		},
		{
			name: "list for loop with global reference fails",
			globals: Globals(
				Expr("list", `["a", "b", "c"]`),
			),
			config: Doc(
				Expr("list", `[for k in global.list : k]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "obj for loop with global reference fails",
			globals: Globals(
				Expr("obj", `{ a = 1}`),
			),
			config: Doc(
				Expr("obj", `[for k in global.obj : k]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "[for in from [for list with global references",
			globals: Globals(
				Expr("list", `["a", "b", "c"]`),
			),
			config: Doc(
				Expr("obj", `[for s in [for s in global.list : s] : s]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "mixing {for and [for",
			config: Doc(
				Expr("obj", `{for k, v in [for k in a.b : k] : k => v}`),
			),
			want: Doc(
				Expr("obj", `{for k, v in [for k in a.b : k] : k => v}`),
			),
		},
		{
			name: "unary operation !",
			config: Doc(
				Expr("num", "!0"),
			),
			want: Doc(
				Expr("num", "!0"),
			),
		},
		{
			name: "unary operation -",
			config: Doc(
				Expr("num", "-0"),
			),
			want: Doc(
				Expr("num", "-0"),
			),
		},
		{
			name: "number indexing",
			config: Doc(
				Expr("a", "b.1000"),
			),
			want: Doc(
				Expr("a", "b.1000"),
			),
		},
		{
			name: "advanced number literal",
			config: Doc(
				Expr("a", "10.1200"),
			),
			want: Doc(
				Expr("a", "10.1200"),
			),
		},
		{
			name: "advanced number literal",
			config: Doc(
				Expr("a", "0.0.A.0"),
			),
			want: Doc(
				Expr("a", "0.0.A.0"),
			),
		},
		{
			name: "parenthesis and splat with newlines",
			config: Doc(
				Expr("a", "(A(). \n*)"),
			),
			want: Doc(
				Expr("a", "(A(). \n*)"),
			),
		},
		{
			name: "funcall and newlines/comments",
			config: Doc(
				Expr("a", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
			want: Doc(
				Expr("a", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
		},
		{
			name: "tm_ funcall and newlines/comments",
			config: Doc(
				Expr("a", "tm_try(\n/**/a\n/**/,/**/b, null/**/\n/**/)"),
			),
			want: Doc(
				Expr("a", "null"),
			),
		},
		{
			name: "objects and newlines/comments",
			config: Doc(
				Expr("a", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
			want: Doc(
				Expr("a", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
		},
		{
			name: "lists and newlines/comments",
			config: Doc(
				Expr("a", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
			want: Doc(
				Expr("a", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
		},
		{
			name: "conditional globals evaluation",
			globals: Globals(
				Str("domain", "mineiros.io"),
				Bool("exists", true),
			),
			config: Doc(
				Expr("a", `global.exists ? global.domain : "example.com"`),
			),
			want: Doc(
				Expr("a", `true ? "mineiros.io" : "example.com"`),
			),
		},
		{
			name: "evaluated empty string in the prefix",
			config: Doc(
				Expr("a", "\"${tm_replace(0,\"0\",\"\")}0\""),
			),
			want: Doc(
				Expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated empty string in the suffix",
			config: Doc(
				Expr("a", "\"0${tm_replace(0,\"0\",\"\")}\""),
			),
			want: Doc(
				Expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines prefix",
			config: Doc(
				Expr("a", "\"${\ntm_replace(0,0,\"\")}0\""),
			),
			want: Doc(
				Expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines suffix",
			config: Doc(
				Expr("a", "\"${tm_replace(0,0,\"\")\n}0\""),
			),
			want: Doc(
				Expr("a", "\"0\""),
			),
		},
		{
			name: "lists and newlines/comments",
			config: Doc(
				Expr("a", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
			want: Doc(
				Expr("a", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
		},
		{
			name: "interpolation advanced 1",
			globals: Globals(
				Str("a", "1"),
			),
			config: Doc(
				Str("a", "0${tm_try(global.a)}2"),
			),
			want: Doc(
				Str("a", "012"),
			),
		},
		{
			name: "escaped interpolation with global reference",
			config: Doc(
				Str("string", `$${global.string}`),
			),
			want: Doc(
				Str("string", "$${global.string}"),
			),
		},
		{
			name: "escaped interpolation with attr",
			config: Doc(
				Str("string", `$${hi}`),
			),
			want: Doc(
				Str("string", "$${hi}"),
			),
		},
		{
			name: "escaped interpolation with number",
			config: Doc(
				Str("string", `$${5}`),
			),
			want: Doc(
				Str("string", "$${5}"),
			),
		},
		{
			name: "empty escaped interpolation",
			config: Doc(
				Str("string", `$${}`),
			),
			want: Doc(
				Str("string", "$${}"),
			),
		},
		{
			name: "escaped interpolation with prefix",
			config: Doc(
				Str("string", `something-$${hi}`),
			),
			want: Doc(
				Str("string", "something-$${hi}"),
			),
		},
		{
			name: "escaped interpolation with suffix",
			config: Doc(
				Str("string", `$${hi}-suffix`),
			),
			want: Doc(
				Str("string", "$${hi}-suffix"),
			),
		},
		{
			name: "nested escaped interpolation",
			config: Doc(
				Str("string", `$${hi$${again}}`),
			),
			want: Doc(
				Str("string", `$${hi$${again}}`),
			),
		},
		{
			name: "interpolation inside escaped interpolation",
			config: Doc(
				Str("string", `$${hi${attr}}`),
			),
			want: Doc(
				Str("string", `$${hi${attr}}`),
			),
		},
		{
			name: "global interpolation inside escaped interpolation",
			globals: Globals(
				Number("a", 666),
			),
			config: Doc(
				Str("string", `$${hi-${global.a}}`),
			),
			want: Doc(
				Str("string", `$${hi-666}`),
			),
		},
		{
			name: "for inside escaped interpolation",
			config: Doc(
				Str("string", `$${[for k in local.a : k]}`),
			),
			want: Doc(
				Str("string", `$${[for k in local.a : k]}`),
			),
		},
		{
			name: "for inside escaped interpolation referencing global",
			config: Doc(
				Str("string", `$${[for k in global.a : k]}`),
			),
			want: Doc(
				Str("string", `$${[for k in global.a : k]}`),
			),
		},
		{
			name: "terramate.path interpolation",
			config: Doc(
				Str("string", `${terramate.stack.path.absolute} test`),
			),
			want: Doc(
				Str("string", `/stack test`),
			),
		},
		{
			name: "huge string as a result of interpolation",
			globals: Globals(
				Str("value", hugestr),
			),
			config: Doc(
				Str("big", "THIS IS ${tm_upper(global.value)} !!!"),
			),
			want: Doc(
				Str("big", fmt.Sprintf("THIS IS %s !!!", strings.ToUpper(hugestr))),
			),
		},
		{
			name: "interpolation eval is empty",
			globals: Globals(
				Str("value", ""),
			),
			config: Doc(
				Str("big", "THIS IS ${tm_upper(global.value)} !!!"),
			),
			want: Doc(
				Str("big", "THIS IS  !!!"),
			),
		},
		{
			name: "interpolation eval is partial",
			globals: Globals(
				Str("value", ""),
			),
			config: Doc(
				Str("test", `THIS IS ${tm_upper(global.value) + "test"} !!!`),
			),
			want: Doc(
				Str("test", `THIS IS ${"" + "test"} !!!`),
			),
		},
		{
			name: "tm_hcl_expression from string",
			config: Doc(
				Expr("a", `tm_hcl_expression("{ a = b }")`),
			),
			want: Doc(
				Expr("a", "{ a = b }"),
			),
		},
		{
			name: "tm_hcl_expression accessing global with interpolation",
			globals: Globals(
				Number("val", 1),
			),
			config: Doc(
				Expr("a", `tm_hcl_expression("data[${global.val}].yay")`),
			),
			want: Doc(
				Expr("a", "data[1].yay"),
			),
		},
		{
			name: "tm_hcl_expression fails if arg is not string",
			config: Doc(
				Expr("a", `tm_hcl_expression([])`),
			),
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			name: "tm_hcl_expression fails if generated expression is invalid",
			config: Doc(
				Expr("a", `tm_hcl_expression("not valid expression")`),
			),
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			name: "tm_ternary with condition with expression",
			globals: Globals(
				Number("val", 1),
			),
			config: Doc(
				Expr("a", `tm_ternary(global.val == 1, 0, 1)`),
			),
			want: Doc(
				Number("a", 0),
			),
		},
		{
			name: "tm_ternary with different result types in branches",
			config: Doc(
				Expr("a", `tm_ternary(true, true, 0)`),
			),
			want: Doc(
				Bool("a", true),
			),
		},
		{
			name: "tm_ternary returning partial result",
			config: Doc(
				Expr("a", "tm_ternary(true, local.var, [])"),
			),
			want: Doc(
				Expr("a", "local.var"),
			),
		},
		{
			name: "tm_ternary returning complete result",
			globals: Globals(
				Str("a", "val"),
			),
			config: Doc(
				Expr("a", `tm_ternary(true, [global.a], [])`),
			),
			want: Doc(
				Expr("a", `["val"]`),
			),
		},
		{
			name: "tm_ternary returning literals",
			config: Doc(
				Expr("a", "tm_ternary(false, local.var, [])"),
			),
			want: Doc(
				Expr("a", "[]"),
			),
		},
		{
			name: "tm_ternary inside deep structures",
			config: Doc(
				Expr("a", `{
					some = {
						deep = {
							structure = {
								value = tm_ternary(true, [local.var], 0)
							}
						}
					}
				}`),
			),
			want: Doc(
				Expr("a", `{
					some = {
						deep = {
							structure = {
								value = [local.var]
							}
						}
					}
				}`),
			),
		},
		{
			name: "tm_ternary mixing tm_ calls with partials",
			config: Doc(
				Expr("a", `tm_ternary(true, {
							evaluated = tm_upper("a")
							partial = local.var
						}, {})`),
			),
			want: Doc(
				Expr("a", `{
							evaluated = "A"
							partial = local.var
						}`),
			),
		},
		{
			name: "tm_ternary mixing globals and unknowns",
			globals: Globals(
				Str("provider", "google"),
			),
			config: Doc(
				Expr("a", `tm_ternary(true, {
							evaluated1 = data.providers[global.provider]
                            evaluated2 = global.provider
							partial = local.var
						}, {})`),
			),
			want: Doc(
				Expr("a", `{
							evaluated1 = data.providers["google"]
							evaluated2 = "google"
							partial = local.var
						}`),
			),
		},
		{
			name: "nested tm_ternary calls with fully evaluated branches",
			config: Doc(
				Expr("a", `tm_ternary(true, tm_ternary(false, "fail", "works"), tm_ternary(true, "fail", "works"))`),
			),
			want: Doc(
				Expr("a", `"works"`),
			),
		},
		{
			name: "nested tm_ternary calls with partial evaluated branches",
			config: Doc(
				Expr("a", `tm_ternary(true, tm_ternary(false, local.fails, local.works), tm_ternary(true, local.fails, local.works))`),
			),
			want: Doc(
				Expr("a", `local.works`),
			),
		},
		{
			name: "nested tm_ternary mixing tm_ calls with partials",
			config: Doc(
				Expr("a", `tm_ternary(true, tm_ternary(true, {
							evaluated = tm_upper("a")
							partial = local.branch1
						}, {}), {})`),
			),
			want: Doc(
				Expr("a", `{
							evaluated = "A"
							partial = local.branch1
						}`),
			),
		},
		{
			name: "nested tm_ternary mixing tm_ calls with partials returning branch2",
			config: Doc(
				Expr("a", `tm_ternary(true, tm_ternary(false, {
							evaluated = tm_upper("a")
							partial = local.branch1
						}, {
							evaluated = tm_upper("a")
							partial = local.branch2
						}), {})`),
			),
			want: Doc(
				Expr("a", `{
							evaluated = "A"
							partial = local.branch2
						}`),
			),
		},
		{
			name: "nested tm_ternary mixing tm_ calls with partials returning branch3",
			config: Doc(
				Expr("a", `tm_ternary(false,
									tm_ternary(false, {
										evaluated = tm_upper("a")
										partial = local.branch1
									}, {
										evaluated = tm_upper("a")
										partial = local.branch2
									}), {
									evaluated = tm_upper("a")
									partial = local.branch3
					})`),
			),
			want: Doc(
				Expr("a", `{
							evaluated = "A"
							partial = local.branch3
						}`),
			),
		},
		{
			name: "tm_ternary fails with partials in the conditions",
			config: Doc(
				Expr("a", "tm_ternary(local.a, true, false)"),
			),
			wantErr: errors.E(eval.ErrPartial),
		},
	}

	for _, tc := range tcases {
		tcase := tc

		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			const genname = "test"
			const stackname = "stack"

			s := sandbox.New(t)
			stackEntry := s.CreateStack(stackname)
			stack := stackEntry.Load(s.Config())
			path := filepath.Join(s.RootDir(), stackname)
			if tcase.globals == nil {
				tcase.globals = Globals()
			}
			hclcfg := Doc(
				tcase.globals,
				GenerateHCL(
					Labels(genname),
					Content(
						tcase.config,
					),
				),
			)

			t.Logf("input: %s", hclcfg.String())
			test.AppendFile(t, path, config.DefaultFilename, hclcfg.String())

			projmeta := s.LoadProjectMetadata()
			root, err := config.LoadRoot(s.RootDir())
			if errors.IsAnyKind(err, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
				errtest.Assert(t, err, tcase.wantErr)
				return
			}

			assert.NoError(t, err)

			cfg, _ := root.Lookup(stack.Path())
			ctx, _ := s.LoadStackGlobals(root, projmeta, stack)
			got, err := genhcl.Load(cfg, ctx)
			errtest.Assert(t, err, tcase.wantErr)
			if err != nil {
				return
			}

			assert.EqualInts(t, len(got), 1, "want single generated HCL")

			gothcl := got[0]
			gotcode := gothcl.Body()
			wantcode := tcase.want.String()
			assertHCLEquals(t, gotcode, wantcode)
		})
	}
}

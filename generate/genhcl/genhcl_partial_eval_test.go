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
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

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
	}

	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}

	hugestr := strings.Repeat("huge ", 1000)
	tcases := []testcase{
		{
			name: "unknown references on attributes",
			config: hcldoc(
				expr("count", "count.index"),
				expr("data", "data.ref"),
				expr("local", "local.ref"),
				expr("module", "module.ref"),
				expr("path", "path.ref"),
				expr("resource", "resource.name.etc"),
				expr("terraform", "terraform.ref"),
			),
			want: hcldoc(
				expr("count", "count.index"),
				expr("data", "data.ref"),
				expr("local", "local.ref"),
				expr("module", "module.ref"),
				expr("path", "path.ref"),
				expr("resource", "resource.name.etc"),
				expr("terraform", "terraform.ref"),
			),
		},
		{
			name: "unknown references on object",
			config: hcldoc(
				expr("obj", `{
					count     = count.index,
					data      = data.ref,
					local     = local.ref,
					module    = module.ref,
					path      = path.ref,
					resource  = resource.name.etc,
					terraform = terraform.ref,
				 }`),
			),
			want: hcldoc(
				expr("obj", `{
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
			globals: hcldoc(
				globals(
					number("ref", 666),
				),
			),
			config: hcldoc(
				expr("obj", `{
					local     = local.ref,
					global    = global.ref,
				 }`),
			),
			want: hcldoc(
				expr("obj", `{
					local   = local.ref,
					global  = 666,
				 }`),
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
				expr("list", `[ local.ref, global.ref ]`),
			),
			want: hcldoc(
				expr("list", `[ local.ref, 666 ]`),
			),
		},
		{
			name: "try with unknown reference on attribute is not evaluated",
			config: hcldoc(
				expr("attr", "try(something.val, null)"),
			),
			want: hcldoc(
				expr("attr", "try(something.val, null)"),
			),
		},
		{
			name: "try with unknown reference on list is not evaluated",
			config: hcldoc(
				expr("list", "[try(something.val, null), 1]"),
			),
			want: hcldoc(
				expr("list", "[try(something.val, null), 1]"),
			),
		},
		{
			name: "try with unknown reference on object is not evaluated",
			config: hcldoc(
				expr("obj", `{
					a = try(something.val, null),	
					b = "val",
				}`),
			),
			want: hcldoc(
				expr("obj", `{
					a = try(something.val, null),	
					b = "val",
				}`),
			),
		},
		{
			name: "variable definition with optionals",
			config: hcldoc(
				variable(
					labels("with_optional_attribute"),
					expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 1)
					})`),
				),
			),
			want: hcldoc(
				variable(
					labels("with_optional_attribute"),
					expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 1)
					})`),
				),
			),
		},
		{
			name: "variable definition with optional default from global",
			globals: hcldoc(
				globals(
					number("default", 666),
				),
			),
			config: hcldoc(
				variable(
					labels("with_optional_attribute"),
					expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, global.default)
					})`),
				),
			),
			want: hcldoc(
				variable(
					labels("with_optional_attribute"),
					expr("type", `object({
					    a = string
					    b = optional(string)
					    c = optional(number, 666)
					})`),
				),
			),
		},
		{
			name: "function call on attr with mixed references is partially evaluated",
			globals: hcldoc(
				globals(
					attr("list", "[1, 2, 3]"),
				),
			),
			config: hcldoc(
				expr("a", "merge(something.val, global.list)"),
				expr("b", "merge(global.list, local.list)"),
			),
			want: hcldoc(
				expr("a", "merge(something.val, [1, 2, 3])"),
				expr("b", "merge([1, 2, 3], local.list)"),
			),
		},
		{
			name: "function call on obj with mixed references is partially evaluated",
			globals: hcldoc(
				globals(
					attr("list", "[1, 2, 3]"),
				),
			),
			config: hcldoc(
				expr("obj", `{
					a = merge(something.val, global.list)
				}`),
			),
			want: hcldoc(
				expr("obj", `{
					a = merge(something.val, [1, 2, 3])
				}`),
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
				str("num", `test-${global.num}`),
			),
			want: hcldoc(
				str("num", "test-1337"),
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
				str("flag", `test-${global.flag}`),
			),
			want: hcldoc(
				str("flag", "test-true"),
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
				str("string", `${global.string}`),
			),
			want: hcldoc(
				str("string", "hello"),
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
				str("string", `test-${global.string}`),
			),
			want: hcldoc(
				str("string", "test-hello"),
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
				str("string", `${global.string}-test`),
			),
			want: hcldoc(
				str("string", "hello-test"),
			),
		},
		{
			name: "multiple variable interpolation with prefixed string",
			globals: globals(
				str("string1", "hello1"),
				str("string2", "hello2"),
			),
			config: hcldoc(
				str("string", `something ${global.string1} and ${global.string2}`),
			),
			want: hcldoc(
				str("string", "something hello1 and hello2"),
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
				str("string", `${global.string1}${global.string2}`),
			),
			want: hcldoc(
				str("string", "hello1hello2"),
			),
		},
		{
			// Here we check that an interpolated object results on the object itself, not a string.
			name: "object interpolation/serialization",
			globals: globals(
				expr("obj", `{
					string = "hello"
					number = 1337
					bool = false
				}`),
			),
			config: hcldoc(
				expr("obj", "global.obj"),
				str("obj_interpolated", "${global.obj}"),
			),
			want: hcldoc(
				expr("obj", `{
					bool = false
					number = 1337
					string = "hello"
				}`),
				expr("obj_interpolated", `{
					bool = false
					number = 1337
					string = "hello"
				}`),
			),
		},
		{
			name: "interpolating multiple objects fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("a", "${global.obj}-${global.obj}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating object with prefix space fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("a", " ${global.obj}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating object with suffix space fails",
			globals: globals(
				expr("obj", `{ string = "hello" }`),
			),
			config: hcldoc(
				str("a", "${global.obj} "),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating multiple lists fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("a", "${global.list}-${global.list}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating list with prefix space fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("a", " ${global.list}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "interpolating list with suffix space fails",
			globals: globals(
				expr("list", `["hello"]`),
			),
			config: hcldoc(
				str("a", "${global.list} "),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			// Here we check that a interpolated lists results on the list itself, not a string.
			name: "list interpolation/serialization",
			globals: globals(
				expr("list", `["hi"]`),
			),
			config: hcldoc(
				expr("list", "global.list"),
				str("list_interpolated", "${global.list}"),
			),
			want: hcldoc(
				expr("list", `["hi"]`),
				expr("list_interpolated", `["hi"]`),
			),
		},
		{
			// Here we check that a interpolated number results on the number itself, not a string.
			name: "number interpolation/serialization",
			globals: globals(
				number("number", 666),
			),
			config: hcldoc(
				expr("number", "global.number"),
				str("number_interpolated", "${global.number}"),
			),
			want: hcldoc(
				number("number", 666),
				number("number_interpolated", 666),
			),
		},
		{
			// Here we check that multiple interpolated numbers results on a string.
			name: "multiple numbers interpolation/serialization",
			globals: globals(
				number("number", 666),
			),
			config: hcldoc(
				expr("number", "global.number"),
				str("number_interpolated", "${global.number}-${global.number}"),
			),
			want: hcldoc(
				number("number", 666),
				str("number_interpolated", "666-666"),
			),
		},
		{
			// Here we check that a interpolated booleans results on the boolean itself, not a string.
			name: "boolean interpolation/serialization",
			globals: globals(
				boolean("bool", true),
			),
			config: hcldoc(
				expr("bool", "global.bool"),
				str("bool_interpolated", "${global.bool}"),
			),
			want: hcldoc(
				boolean("bool", true),
				boolean("bool_interpolated", true),
			),
		},
		{
			// Here we check that multiple interpolated booleans results on a string.
			name: "multiple booleans interpolation/serialization",
			globals: globals(
				boolean("bool", true),
			),
			config: hcldoc(
				expr("bool", "global.bool"),
				str("bool_interpolated", "${global.bool}-${global.bool}"),
			),
			want: hcldoc(
				boolean("bool", true),
				str("bool_interpolated", "true-true"),
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
				str("var", "${global.interp}"),
			),
			want: hcldoc(
				expr("var", "[1, 2, 3]"),
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
				str("var", "${global.list}"),
			),
			want: hcldoc(
				expr("var", "[1, 2, 3]"),
			),
		},
		{
			name: "basic plus expression",
			config: hcldoc(
				expr("var", `1 + 1`),
			),
			want: hcldoc(
				expr("var", `1 + 1`),
			),
		},
		{
			name: "plus expression funcall",
			config: hcldoc(
				expr("var", `len(a.b) + len2(c.d)`),
			),
			want: hcldoc(
				expr("var", `len(a.b) + len2(c.d)`),
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
				expr("var", `tm_upper(global.a) + tm_upper(global.b)`),
			),
			want: hcldoc(
				expr("var", `"HELLO" + "WORLD"`),
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
				expr("var", `tm_lower(tm_upper(global.a)) + tm_lower(tm_upper(global.b))`),
			),
			want: hcldoc(
				expr("var", `"hello" + "world"`),
			),
		},
		{
			name: "basic minus expression",
			config: hcldoc(
				expr("var", `1 + 1`),
			),
			want: hcldoc(
				expr("var", `1 + 1`),
			),
		},
		{
			name: "conditional expression",
			config: hcldoc(
				expr("var", `1 == 1 ? 0 : 1`),
			),
			want: hcldoc(
				expr("var", `1 == 1 ? 0 : 1`),
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
				expr("var", `1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: hcldoc(
				expr("var", `1 >= 10 ? local.x : [for x in local.a : x]`),
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
				expr("var", `local.x + 1 >= global.num ? local.x : [for x in local.a : x]`),
			),
			want: hcldoc(
				expr("var", `local.x + 1 >= 10 ? local.x : [for x in local.a : x]`),
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
				str("var", "${global.obj.string} ${global.obj.obj2.obj3.number}"),
			),
			want: hcldoc(
				str("var", "hello 1337"),
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
				str("var", "${global.obj.string} ${global.obj.obj2.obj3}"),
			),
			wantErr: errors.E(eval.ErrInterpolation),
		},
		{
			name: "basic list indexing",
			globals: hcldoc(
				globals(
					expr("list", `["a", "b", "c"]`),
				),
			),
			config: hcldoc(
				expr("string", `global.list[0]`),
			),
			want: hcldoc(
				str("string", "a"),
			),
		},
		{
			name: "advanced list indexing",
			globals: hcldoc(
				globals(
					expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: hcldoc(
				expr("num", `global.list[1][1]`),
			),
			want: hcldoc(
				number("num", 5),
			),
		},
		{
			name: "advanced list indexing 2",
			globals: hcldoc(
				globals(
					expr("list", `[ [1, 2, 3], [4, 5, 6], [7, 8, 9]]`),
				),
			),
			config: hcldoc(
				expr("num", `global.list[1+1][1-1]`),
			),
			want: hcldoc(
				number("num", 7),
			),
		},
		{
			name: "advanced object indexing",
			globals: hcldoc(
				globals(
					expr("obj", `{A = {B = "test"}}`),
				),
			),
			config: hcldoc(
				expr("string", `global.obj[tm_upper("a")][tm_upper("b")]`),
			),
			want: hcldoc(
				str("string", "test"),
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
				expr("string", `global.obj["a"]`),
			),
			want: hcldoc(
				str("string", "b"),
			),
		},
		{
			name: "indexing of outside variables",
			globals: hcldoc(
				globals(
					number("depth", 1),
				),
			),
			config: hcldoc(
				expr("folder_id", `data.google_active_folder[global.depth].0.id`),
			),
			want: hcldoc(
				expr("folder_id", `data.google_active_folder[1].0.id`),
			),
		},
		{
			name: "indexing of outside variables with interpolation of single var",
			globals: hcldoc(
				globals(
					number("depth", 1),
				),
			),
			config: hcldoc(
				expr("folder_id", `data.google_active_folder["${global.depth}"].0.id`),
			),
			want: hcldoc(
				expr("folder_id", `data.google_active_folder[1].0.id`),
			),
		},
		{
			name: "indexing of outside variables with interpolation",
			globals: hcldoc(
				globals(
					number("depth", 1),
				),
			),
			config: hcldoc(
				expr("folder_id", `data.google_active_folder["l${global.depth}"].0.id`),
			),
			want: hcldoc(
				expr("folder_id", `data.google_active_folder["l1"].0.id`),
			),
		},
		{
			name: "outside variable with splat operator",
			config: hcldoc(
				expr("folder_id", `data.test[*].0.id`),
			),
			want: hcldoc(
				expr("folder_id", `data.test[*].0.id`),
			),
		},
		{
			name: "outside variable with splat getattr operator",
			config: hcldoc(
				expr("folder_id", `data.test.*.0.id`),
			),
			want: hcldoc(
				expr("folder_id", `data.test.*.0.id`),
			),
		},
		{
			name: "multiple indexing",
			config: hcldoc(
				expr("a", `data.test[0][0][0]`),
			),
			want: hcldoc(
				expr("a", `data.test[0][0][0]`),
			),
		},
		{
			name: "multiple indexing with evaluation",
			globals: hcldoc(
				globals(
					number("val", 1),
				),
			),
			config: hcldoc(
				expr("a", `data.test[global.val][0][0]`),
			),
			want: hcldoc(
				expr("a", `data.test[1][0][0]`),
			),
		},
		{
			name: "multiple indexing with evaluation 2",
			globals: hcldoc(
				globals(
					number("val", 1),
				),
			),
			config: hcldoc(
				expr("a", `data.test[0][global.val][global.val+1]`),
			),
			want: hcldoc(
				expr("a", `data.test[0][1][1+1]`),
			),
		},
		{
			name: "nested indexing",
			globals: hcldoc(
				globals(
					expr("obj", `{
						key = {
							key2 = {
								val = "hello"
							}
						}
					}`),
					expr("obj2", `{
						keyname = "key"
					}`),
					expr("key", `{
						key2 = "keyname"
					}`),
					str("key2", "key2"),
				),
			),
			config: hcldoc(
				expr("hello", `global.obj[global.obj2[global.key[global.key2]]][global.key2]["val"]`),
			),
			want: hcldoc(
				str("hello", "hello"),
			),
		},
		{
			name: "obj for loop without eval references",
			config: hcldoc(
				expr("obj", `{for k in local.list : k => k}`),
			),
			want: hcldoc(
				expr("obj", `{for k in local.list : k => k}`),
			),
		},
		{
			name: "list for loop without eval references",
			config: hcldoc(
				expr("obj", `[for k in local.list : k]`),
			),
			want: hcldoc(
				expr("obj", `[for k in local.list : k]`),
			),
		},
		{
			name: "{for loop from map and funcall",
			config: hcldoc(
				expr("obj", `{for s in var.list : s => upper(s)}`),
			),
			want: hcldoc(
				expr("obj", `{for s in var.list : s => upper(s)}`),
			),
		},
		{
			name: "{for in from {for map",
			config: hcldoc(
				expr("obj", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
			want: hcldoc(
				expr("obj", `{for k, v in {for k,v in a.b : k=>v} : k => v}`),
			),
		},
		{
			name: "[for with funcall",
			config: hcldoc(
				expr("obj", `[for s in var.list : upper(s)]`),
			),
			want: hcldoc(
				expr("obj", `[for s in var.list : upper(s)]`),
			),
		},
		{
			name: "[for in from map and Operation body",
			config: hcldoc(
				expr("obj", `[for k, v in var.map : length(k) + length(v)]`),
			),
			want: hcldoc(
				expr("obj", `[for k, v in var.map : length(k) + length(v)]`),
			),
		},
		{
			name: "[for in from map and interpolation body",
			config: hcldoc(
				expr("obj", `[for i, v in var.list : "${i} is ${v}"]`),
			),
			want: hcldoc(
				expr("obj", `[for i, v in var.list : "${i} is ${v}"]`),
			),
		},
		{
			name: "[for in from map with conditional body",
			config: hcldoc(
				expr("obj", `[for s in var.list : upper(s) if s != ""]`),
			),
			want: hcldoc(
				expr("obj", `[for s in var.list : upper(s) if s != ""]`),
			),
		},
		{
			name: "[for in from [for list",
			config: hcldoc(
				expr("obj", `[for s in [for s in a.b : s] : s]`),
			),
			want: hcldoc(
				expr("obj", `[for s in [for s in a.b : s] : s]`),
			),
		},
		{
			name: "list for loop with global reference fails",
			globals: globals(
				expr("list", `["a", "b", "c"]`),
			),
			config: hcldoc(
				expr("list", `[for k in global.list : k]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "obj for loop with global reference fails",
			globals: globals(
				expr("obj", `{ a = 1}`),
			),
			config: hcldoc(
				expr("obj", `[for k in global.obj : k]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "[for in from [for list with global references",
			globals: globals(
				expr("list", `["a", "b", "c"]`),
			),
			config: hcldoc(
				expr("obj", `[for s in [for s in global.list : s] : s]`),
			),
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			name: "mixing {for and [for",
			config: hcldoc(
				expr("obj", `{for k, v in [for k in a.b : k] : k => v}`),
			),
			want: hcldoc(
				expr("obj", `{for k, v in [for k in a.b : k] : k => v}`),
			),
		},
		{
			name: "unary operation !",
			config: hcldoc(
				expr("num", "!0"),
			),
			want: hcldoc(
				expr("num", "!0"),
			),
		},
		{
			name: "unary operation -",
			config: hcldoc(
				expr("num", "-0"),
			),
			want: hcldoc(
				expr("num", "-0"),
			),
		},
		{
			name: "number indexing",
			config: hcldoc(
				expr("a", "b.1000"),
			),
			want: hcldoc(
				expr("a", "b.1000"),
			),
		},
		{
			name: "advanced number literal",
			config: hcldoc(
				expr("a", "10.1200"),
			),
			want: hcldoc(
				expr("a", "10.1200"),
			),
		},
		{
			name: "advanced number literal",
			config: hcldoc(
				expr("a", "0.0.A.0"),
			),
			want: hcldoc(
				expr("a", "0.0.A.0"),
			),
		},
		{
			name: "parenthesis and splat with newlines",
			config: hcldoc(
				expr("a", "(A(). \n*)"),
			),
			want: hcldoc(
				expr("a", "(A(). \n*)"),
			),
		},
		{
			name: "funcall and newlines/comments",
			config: hcldoc(
				expr("a", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
			want: hcldoc(
				expr("a", "funcall(\n/**/a\n/**/,/**/b/**/\n/**/)"),
			),
		},
		{
			name: "tm_ funcall and newlines/comments",
			config: hcldoc(
				expr("a", "tm_try(\n/**/a\n/**/,/**/b, null/**/\n/**/)"),
			),
			want: hcldoc(
				expr("a", "null"),
			),
		},
		{
			name: "objects and newlines/comments",
			config: hcldoc(
				expr("a", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
			want: hcldoc(
				expr("a", "{/**/\n/**/a/**/=/**/\"a\"/**/\n}"),
			),
		},
		{
			name: "lists and newlines/comments",
			config: hcldoc(
				expr("a", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
			want: hcldoc(
				expr("a", "[/**/\n/**/a/**/\n,\"a\"/**/\n]"),
			),
		},
		{
			name: "conditional globals evaluation",
			globals: globals(
				str("domain", "mineiros.io"),
				boolean("exists", true),
			),
			config: hcldoc(
				expr("a", `global.exists ? global.domain : "example.com"`),
			),
			want: hcldoc(
				expr("a", `true ? "mineiros.io" : "example.com"`),
			),
		},
		{
			name: "evaluated empty string in the prefix",
			config: hcldoc(
				expr("a", "\"${tm_replace(0,\"0\",\"\")}0\""),
			),
			want: hcldoc(
				expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated empty string in the suffix",
			config: hcldoc(
				expr("a", "\"0${tm_replace(0,\"0\",\"\")}\""),
			),
			want: hcldoc(
				expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines prefix",
			config: hcldoc(
				expr("a", "\"${\ntm_replace(0,0,\"\")}0\""),
			),
			want: hcldoc(
				expr("a", "\"0\""),
			),
		},
		{
			name: "evaluated funcall with newlines suffix",
			config: hcldoc(
				expr("a", "\"${tm_replace(0,0,\"\")\n}0\""),
			),
			want: hcldoc(
				expr("a", "\"0\""),
			),
		},
		{
			name: "lists and newlines/comments",
			config: hcldoc(
				expr("a", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
			want: hcldoc(
				expr("a", "[/**/\n/**/1/**/\n/**/,/**/\n/**/2/**/\n]"),
			),
		},
		{
			name: "interpolation advanced 1",
			globals: globals(
				str("a", "1"),
			),
			config: hcldoc(
				str("a", "0${tm_try(global.a)}2"),
			),
			want: hcldoc(
				str("a", "012"),
			),
		},
		{
			name: "escaped interpolation with global reference",
			config: hcldoc(
				str("string", `$${global.string}`),
			),
			want: hcldoc(
				str("string", "$${global.string}"),
			),
		},
		{
			name: "escaped interpolation with attr",
			config: hcldoc(
				str("string", `$${hi}`),
			),
			want: hcldoc(
				str("string", "$${hi}"),
			),
		},
		{
			name: "escaped interpolation with number",
			config: hcldoc(
				str("string", `$${5}`),
			),
			want: hcldoc(
				str("string", "$${5}"),
			),
		},
		{
			name: "empty escaped interpolation",
			config: hcldoc(
				str("string", `$${}`),
			),
			want: hcldoc(
				str("string", "$${}"),
			),
		},
		{
			name: "escaped interpolation with prefix",
			config: hcldoc(
				str("string", `something-$${hi}`),
			),
			want: hcldoc(
				str("string", "something-$${hi}"),
			),
		},
		{
			name: "escaped interpolation with suffix",
			config: hcldoc(
				str("string", `$${hi}-suffix`),
			),
			want: hcldoc(
				str("string", "$${hi}-suffix"),
			),
		},
		{
			name: "nested escaped interpolation",
			config: hcldoc(
				str("string", `$${hi$${again}}`),
			),
			want: hcldoc(
				str("string", `$${hi$${again}}`),
			),
		},
		{
			name: "interpolation inside escaped interpolation",
			config: hcldoc(
				str("string", `$${hi${attr}}`),
			),
			want: hcldoc(
				str("string", `$${hi${attr}}`),
			),
		},
		{
			name: "global interpolation inside escaped interpolation",
			globals: globals(
				number("a", 666),
			),
			config: hcldoc(
				str("string", `$${hi-${global.a}}`),
			),
			want: hcldoc(
				str("string", `$${hi-666}`),
			),
		},
		{
			name: "for inside escaped interpolation",
			config: hcldoc(
				str("string", `$${[for k in local.a : k]}`),
			),
			want: hcldoc(
				str("string", `$${[for k in local.a : k]}`),
			),
		},
		{
			name: "for inside escaped interpolation referencing global",
			config: hcldoc(
				str("string", `$${[for k in global.a : k]}`),
			),
			want: hcldoc(
				str("string", `$${[for k in global.a : k]}`),
			),
		},
		{
			name: "terramate.path interpolation",
			config: hcldoc(
				str("string", `${terramate.stack.path.absolute} test`),
			),
			want: hcldoc(
				str("string", `/stack test`),
			),
		},
		{
			name: "huge string as a result of interpolation",
			globals: globals(
				str("value", hugestr),
			),
			config: hcldoc(
				str("big", "THIS IS ${tm_upper(global.value)} !!!"),
			),
			want: hcldoc(
				str("big", fmt.Sprintf("THIS IS %s !!!", strings.ToUpper(hugestr))),
			),
		},
		{
			name: "interpolation eval is empty",
			globals: globals(
				str("value", ""),
			),
			config: hcldoc(
				str("big", "THIS IS ${tm_upper(global.value)} !!!"),
			),
			want: hcldoc(
				str("big", "THIS IS  !!!"),
			),
		},
		{
			name: "interpolation eval is partial",
			globals: globals(
				str("value", ""),
			),
			config: hcldoc(
				str("test", `THIS IS ${tm_upper(global.value) + "test"} !!!`),
			),
			want: hcldoc(
				str("test", `THIS IS ${"" + "test"} !!!`),
			),
		},
		{
			name: "tm_ternary with condition with expression",
			globals: globals(
				number("val", 1),
			),
			config: hcldoc(
				expr("a", `tm_ternary(global.val == 1, 0, 1)`),
			),
			want: hcldoc(
				number("a", 0),
			),
		},
		{
			name: "tm_ternary with different result types in branches",
			config: hcldoc(
				expr("a", `tm_ternary(true, true, 0)`),
			),
			want: hcldoc(
				boolean("a", true),
			),
		},
		{
			name: "tm_ternary returning partial result",
			config: hcldoc(
				expr("a", "tm_ternary(true, local.var, [])"),
			),
			want: hcldoc(
				expr("a", "local.var"),
			),
		},
		{
			name: "tm_ternary returning literals",
			config: hcldoc(
				expr("a", "tm_ternary(false, local.var, [])"),
			),
			want: hcldoc(
				expr("a", "[]"),
			),
		},
		{
			name: "tm_ternary inside deep structures",
			config: hcldoc(
				expr("a", `{
					some = {
						deep = {
							structure = {
								value = tm_ternary(true, [local.var], 0)
							}
						}
					}	
				}`),
			),
			want: hcldoc(
				expr("a", `{
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
			name: "tm_ternary fails with partials in the conditions",
			config: hcldoc(
				expr("a", "tm_ternary(local.a, true, false)"),
			),
			wantErr: errors.E(eval.ErrPartial),
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
			const genname = "test"
			const stackname = "stack"

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
					content(
						tcase.config,
					),
				),
			)

			t.Logf("input: %s", cfg.String())
			test.AppendFile(t, path, config.DefaultFilename, cfg.String())

			projmeta := s.LoadProjectMetadata()
			globals := s.LoadStackGlobals(projmeta, stack)
			got, err := genhcl.Load(projmeta, stack, globals)
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

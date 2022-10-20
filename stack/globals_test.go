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

package stack_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"

	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"

	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/zclconf/go-cty-debug/ctydebug"
)

// TODO(katcipis): add tests related to tf functions that depend on filesystem
// (BaseDir parameter passed on Scope when creating eval context).
func TestLoadGlobals(t *testing.T) {
	type (
		hclconfig struct {
			path     string
			filename string
			add      *hclwrite.Block
		}
		testcase struct {
			name    string
			layout  []string
			configs []hclconfig
			want    map[string]*hclwrite.Block
			wantErr error
		}
	)

	tcases := []testcase{
		{
			name:   "no stacks no globals",
			layout: []string{},
		},
		{
			name:   "single stacks no globals",
			layout: []string{"s:stack"},
		},
		{
			name: "two stacks no globals",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name: "non-global block is ignored",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  Block("terramate"),
				},
			},
		},
		{
			name:   "single stack with its own globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("some_string", "string"),
						Number("some_number", 777),
						Bool("some_bool", true),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("some_string", "string"),
					Number("some_number", 777),
					Bool("some_bool", true),
				),
			},
		},
		{
			name:   "single stack with three globals blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{path: "/stack", add: Globals(Str("str", "hi"))},
				{path: "/stack", add: Globals(Number("num", 666))},
				{path: "/stack", add: Globals(Bool("bool", false))},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("str", "hi"),
					Number("num", 666),
					Bool("bool", false),
				),
			},
		},
		{
			name: "multiple stacks with config on parent dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{path: "/stacks", add: Globals(Str("parent", "hi"))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(Str("parent", "hi")),
				"/stacks/stack-2": Globals(Str("parent", "hi")),
			},
		},
		{
			name: "multiple stacks with config on root dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{path: "/", add: Globals(Str("root", "hi"))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(Str("root", "hi")),
				"/stacks/stack-2": Globals(Str("root", "hi")),
			},
		},
		{
			name: "multiple stacks merging no overriding",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{path: "/", add: Globals(Str("root", "root"))},
				{path: "/stacks", add: Globals(Bool("parent", true))},
				{path: "/stacks/stack-1", add: Globals(Number("stack", 666))},
				{path: "/stacks/stack-2", add: Globals(Number("stack", 777))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("root", "root"),
					Bool("parent", true),
					Number("stack", 666),
				),
				"/stacks/stack-2": Globals(
					Str("root", "root"),
					Bool("parent", true),
					Number("stack", 777),
				),
			},
		},
		{
			name: "multiple stacks merging with overriding",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("field_a", "field_a_root"),
						Str("field_b", "field_b_root"),
					),
				},
				{
					path: "/stacks",
					add: Globals(
						Str("field_b", "field_b_stacks"),
						Str("field_c", "field_c_stacks"),
						Str("field_d", "field_d_stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("field_a", "field_a_stack_1"),
						Str("field_b", "field_b_stack_1"),
						Str("field_c", "field_c_stack_1"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Str("field_d", "field_d_stack_2"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("field_a", "field_a_stack_1"),
					Str("field_b", "field_b_stack_1"),
					Str("field_c", "field_c_stack_1"),
					Str("field_d", "field_d_stacks"),
				),
				"/stacks/stack-2": Globals(
					Str("field_a", "field_a_root"),
					Str("field_b", "field_b_stacks"),
					Str("field_c", "field_c_stacks"),
					Str("field_d", "field_d_stack_2"),
				),
				"/stacks/stack-3": Globals(
					Str("field_a", "field_a_root"),
					Str("field_b", "field_b_stacks"),
					Str("field_c", "field_c_stacks"),
					Str("field_d", "field_d_stacks"),
				),
			},
		},
		{
			name: "stacks referencing all metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2:id=stack-2-id;description=someDescriptionStack2",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-1",
					add: Globals(
						Expr("stacks_list", "terramate.stacks.list"),
						Expr("stack_path_abs", "terramate.stack.path.absolute"),
						Expr("stack_path_rel", "terramate.stack.path.relative"),
						Expr("stack_path_to_root", "terramate.stack.path.to_root"),
						Expr("stack_path_basename", "terramate.stack.path.basename"),
						Expr("stack_id", `tm_try(terramate.stack.id, "no-id")`),
						Expr("stack_name", "terramate.stack.name"),
						Expr("stack_description", "terramate.stack.description"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Expr("stacks_list", "terramate.stacks.list"),
						Expr("stack_path_abs", "terramate.stack.path.absolute"),
						Expr("stack_path_rel", "terramate.stack.path.relative"),
						Expr("stack_path_to_root", "terramate.stack.path.to_root"),
						Expr("stack_path_basename", "terramate.stack.path.basename"),
						Expr("stack_id", "terramate.stack.id"),
						Expr("stack_name", "terramate.stack.name"),
						Expr("stack_description", "terramate.stack.description"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					EvalExpr(t, "stacks_list", `tolist(["/stacks/stack-1", "/stacks/stack-2"])`),
					Str("stack_path_abs", "/stacks/stack-1"),
					Str("stack_path_rel", "stacks/stack-1"),
					Str("stack_path_to_root", "../.."),
					Str("stack_path_basename", "stack-1"),
					Str("stack_id", "no-id"),
					Str("stack_name", "stack-1"),
					Str("stack_description", ""),
				),
				"/stacks/stack-2": Globals(
					EvalExpr(t, "stacks_list", `tolist(["/stacks/stack-1", "/stacks/stack-2"])`),
					Str("stack_path_abs", "/stacks/stack-2"),
					Str("stack_path_rel", "stacks/stack-2"),
					Str("stack_path_to_root", "../.."),
					Str("stack_path_basename", "stack-2"),
					Str("stack_id", "stack-2-id"),
					Str("stack_name", "stack-2"),
					Str("stack_description", "someDescriptionStack2"),
				),
			},
		},
		{
			name: "stacks using functions and metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-1",
					add: Globals(
						Expr("interpolated", `"prefix-${tm_replace(terramate.stack.path.absolute, "/", "@")}-suffix"`),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Expr("stack_path", `tm_replace(terramate.stack.path.absolute, "/", "-")`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("interpolated", "prefix-@stacks@stack-1-suffix"),
				),
				"/stacks/stack-2": Globals(Str("stack_path", "-stacks-stack-2")),
			},
		},
		{
			name:   "stack with globals referencing globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("field", "some-string"),
						Expr("stack_path", "terramate.stack.path.absolute"),
						Expr("ref_field", "global.field"),
						Expr("ref_stack_path", "global.stack_path"),
						Expr("interpolation", `"${global.ref_stack_path}-${global.ref_field}"`),
						Expr("ref_interpolation", "global.interpolation"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("field", "some-string"),
					Str("stack_path", "/stack"),
					Str("ref_field", "some-string"),
					Str("ref_stack_path", "/stack"),
					Str("interpolation", "/stack-some-string"),
					Str("ref_interpolation", "/stack-some-string"),
				),
			},
		},
		{
			name:   "stack with globals referencing globals on multiple files",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals_1.tm.hcl",
					add: Globals(
						Str("field", "some-string"),
						Expr("stack_path", "terramate.stack.path.absolute"),
					),
				},
				{
					path:     "/stack",
					filename: "globals_2.tm.hcl",
					add: Globals(
						Expr("ref_field", "global.field"),
						Expr("ref_stack_path", "global.stack_path"),
					),
				},
				{
					path:     "/stack",
					filename: "globals_3.tm.hcl",
					add: Globals(
						Expr("interpolation", `"${global.ref_stack_path}-${global.ref_field}"`),
						Expr("ref_interpolation", "global.interpolation"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("field", "some-string"),
					Str("stack_path", "/stack"),
					Str("ref_field", "some-string"),
					Str("ref_stack_path", "/stack"),
					Str("interpolation", "/stack-some-string"),
					Str("ref_interpolation", "/stack-some-string"),
				),
			},
		},
		{
			name: "root with globals referencing globals on multiple files",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/",
					filename: "globals_1.tm.hcl",
					add: Globals(
						Str("field", "some-string"),
						Expr("stack_path", "terramate.stack.path.absolute"),
					),
				},
				{
					path:     "/",
					filename: "globals_2.tm.hcl",
					add: Globals(
						Expr("ref_field", "global.field"),
						Expr("ref_stack_path", "global.stack_path"),
					),
				},
				{
					path:     "/",
					filename: "globals_3.tm.hcl",
					add: Globals(
						Expr("interpolation", `"${global.ref_stack_path}-${global.ref_field}"`),
						Expr("ref_interpolation", "global.interpolation"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("field", "some-string"),
					Str("stack_path", "/stacks/stack-1"),
					Str("ref_field", "some-string"),
					Str("ref_stack_path", "/stacks/stack-1"),
					Str("interpolation", "/stacks/stack-1-some-string"),
					Str("ref_interpolation", "/stacks/stack-1-some-string"),
				),
				"/stacks/stack-2": Globals(
					Str("field", "some-string"),
					Str("stack_path", "/stacks/stack-2"),
					Str("ref_field", "some-string"),
					Str("ref_stack_path", "/stacks/stack-2"),
					Str("interpolation", "/stacks/stack-2-some-string"),
					Str("ref_interpolation", "/stacks/stack-2-some-string"),
				),
			},
		},
		{
			name:   "stack with globals referencing globals hierarchically no overriding",
			layout: []string{"s:envs/prod/stacks/stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("root_field", "root-data"),
						Number("root_number", 666),
						Bool("root_bool", true),
						Expr("root_stack_ref", "global.stack_inter"),
					),
				},
				{
					path: "/envs",
					add: Globals(
						Expr("env_metadata", "terramate.stack.path.absolute"),
						Expr("env_root_ref", "global.root_field"),
					),
				},
				{
					path: "/envs/prod",
					add:  Globals(Str("env", "prod")),
				},
				{
					path: "/envs/prod/stacks",
					add: Globals(
						Expr("stacks_field", `"${terramate.stack.name}-${global.env}"`),
					),
				},
				{
					path: "/envs/prod/stacks/stack",
					add: Globals(
						Expr("stack_inter", `"${global.root_field}-${global.env}-${global.stacks_field}"`),
						Expr("stack_bool", "global.root_bool"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/envs/prod/stacks/stack": Globals(
					Str("root_field", "root-data"),
					Number("root_number", 666),
					Bool("root_bool", true),
					Str("root_stack_ref", "root-data-prod-stack-prod"),
					Str("env_metadata", "/envs/prod/stacks/stack"),
					Str("env_root_ref", "root-data"),
					Str("env", "prod"),
					Str("stacks_field", "stack-prod"),
					Str("stack_inter", "root-data-prod-stack-prod"),
					Bool("stack_bool", true),
				),
			},
		},
		{
			name: "stack with globals referencing globals hierarchically and overriding",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("stack_ref", "global.stack"),
					),
				},
				{
					path: "/stacks",
					add: Globals(
						Expr("stack_ref", "global.stack_other"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("stack", "stack-1"),
						Str("stack_other", "other stack-1"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Str("stack", "stack-2"),
						Str("stack_other", "other stack-2"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("stack", "stack-1"),
					Str("stack_other", "other stack-1"),
					Str("stack_ref", "other stack-1"),
				),
				"/stacks/stack-2": Globals(
					Str("stack", "stack-2"),
					Str("stack_other", "other stack-2"),
					Str("stack_ref", "other stack-2"),
				),
			},
		},
		{
			name: "single stack extending local globals",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							EvalExpr(t, "obj", `{}`),
						),
						Globals(
							Labels("obj"),
							Number("number", 1),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{ number = 1 }`),
				),
			},
		},
		{
			name: "stack extending local globals - order does not matter",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("number", 1),
						),
						Globals(
							EvalExpr(t, "obj", `{}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{ number = 1 }`),
				),
			},
		},
		{
			name: "single stack extending nested local global",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							EvalExpr(t, "a", `{test = 1}`),
						),
						Globals(
							Labels("a"),
							EvalExpr(t, "b", `{test = 1}`),
						),
						Globals(
							Labels("a", "b"),
							EvalExpr(t, "c", `{test = 1}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{
						test = 1
						b = {
							test = 1
							c = {
								test = 1
							}
						}
					}`),
				),
			},
		},
		{
			name: "extending funcall resulted object",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("a", `tm_merge({test1 = 1}, {test2 = 1})`),
						),
						Globals(
							Labels("a"),
							EvalExpr(t, "b", `{test = 1}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{
						test1 = 1
						test2 = 1
						b = {
							test = 1
						}
					}`),
				),
			},
		},
		{
			name: "extending non-existent objects",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("a", "b", "c", "d"),
							Number("number", 1),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{
						b = {
							c = {
								d = {
									number = 1
								}
							}
						}
					}`),
				),
			},
		},
		{
			name: "inheriting labelled globals without attributes",
			layout: []string{
				"s:stacks/stack-a",
				"s:stacks/stack-b",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							Labels("obj"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
					}`),
				),
				"/stacks/stack-b": Globals(
					EvalExpr(t, "obj", `{
					}`),
				),
			},
		},
		{
			name: "empty labeled globals do not overwrite existing ones",
			layout: []string{
				"s:stacks/stack-a",
				"s:stacks/stack-b",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("a", 1),
						),

						Globals(
							Labels("obj"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
						a = 1
					}`),
				),
				"/stacks/stack-b": Globals(
					EvalExpr(t, "obj", `{
						a = 1
					}`),
				),
			},
		},
		{
			name: "child scopes can overwrite extended globals",
			layout: []string{
				"s:stacks/stack-a",
				"s:stacks/stack-b",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("a", 1),
						),
					),
				},
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							Expr("obj", `{
									b = 2
								}
							`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
						b = 2
					}`),
				),
				"/stacks/stack-b": Globals(
					EvalExpr(t, "obj", `{
						a = 1
					}`),
				),
			},
		},
		{
			name: "extending globals from parent scope",
			layout: []string{
				"s:stacks/stack-a",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							EvalExpr(t, "obj", `{}`),
						),
						Globals(
							Labels("obj"),
							Number("number", 1),
						),
					),
				},
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("another_number", 2),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
						number = 1
						another_number = 2
					}`),
				),
			},
		},
		{
			name: "parent scope extending globals from stacks - lazy extend",
			layout: []string{
				"s:stacks/stack-a",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("number", 1),
						),
					),
				},
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							EvalExpr(t, "obj", `{
								name = "stack"
							}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
						number = 1
						name = "stack"
					}`),
				),
			},
		},
		{
			name: "extending with a conflict - fails",
			layout: []string{
				"s:stacks/stack-a",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("number", 1),
						),
						Globals(
							Labels("obj"),
							Number("number", 10),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name: "child scope can redefine same key paths",
			layout: []string{
				"s:stacks/stack-a",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("number", 1),
							Str("string", "test"),
						),
					),
				},
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							Labels("obj"),
							Number("number", 100),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-a": Globals(
					EvalExpr(t, "obj", `{
						number = 100
						string = "test"
					}`),
				),
			},
		},
		{
			name: "extending list fails",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							EvalExpr(t, "lst", `[]`),
						),
						Globals(
							Labels("lst"),
							EvalExpr(t, "values", `[1, 2]`),
						),
					),
				},
			},
			wantErr: errors.E(eval.ErrCannotExtendObject),
		},
		{
			name: "extending non-objects fails",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Number("number", 1),
						),
						Globals(
							Labels("number"),
							Str("string", "value"),
						),
					),
				},
			},
			wantErr: errors.E(eval.ErrCannotExtendObject),
		},
		{
			name: "extending nested literal object",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							EvalExpr(t, "a", `{
								b = {

								}
							}`),
						),
						Globals(
							Labels("a", "b"),
							Number("number", 1),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{
						b = {
							number = 1
						}
					}`),
				),
			},
		},
		{
			name: "nested literal object with a conflict",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							EvalExpr(t, "a", `{
								b = {
									number = 1
								}
							}`),
						),
						Globals(
							Labels("a", "b"),
							Number("number", 100),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name: "funcall object with a conflict",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("a", `tm_merge({
								b = {
									number = 1
								}
							}, {a = 1})`),
						),
						Globals(
							Labels("a", "b"),
							Number("number", 100),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name: "globals hierarchically defined with different filenames",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/",
					filename: "root_globals.tm",
					add: Globals(
						Expr("stack_ref", "global.stack"),
					),
				},
				{
					path:     "/stacks",
					filename: "stacks_globals.tm.hcl",
					add: Globals(
						Expr("stack_ref", "global.stack_other"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "stack_1_globals.tm",
					add: Globals(
						Str("stack", "stack-1"),
						Str("stack_other", "other stack-1"),
					),
				},
				{
					path:     "/stacks/stack-2",
					filename: "stack_2_globals.tm.hcl",
					add: Globals(
						Str("stack", "stack-2"),
						Str("stack_other", "other stack-2"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("stack", "stack-1"),
					Str("stack_other", "other stack-1"),
					Str("stack_ref", "other stack-1"),
				),
				"/stacks/stack-2": Globals(
					Str("stack", "stack-2"),
					Str("stack_other", "other stack-2"),
					Str("stack_ref", "other stack-2"),
				),
			},
		},
		{
			name:   "unknown global reference is ignored if it is overridden",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add:  Globals(Expr("field", "global.wont_exist")),
				},
				{
					path: "/stack",
					add:  Globals(Str("field", "data")),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(Str("field", "data")),
			},
		},
		{
			name:   "global reference with functions",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add:  Globals(Str("field", "@lala@hello")),
				},
				{
					path: "/stack",
					add: Globals(
						Expr("newfield", `tm_replace(global.field, "@", "/")`),
						Expr("splitfun", `tm_split("@", global.field)[1]`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("field", "@lala@hello"),
					Str("newfield", "/lala/hello"),
					Str("splitfun", "lala"),
				),
			},
		},
		{
			name:   "global reference with successful tm_try on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "team", `{ members = ["aaa"] }`),
						Expr("members", "global.team.members"),
						Expr("members_try", `tm_try(global.team.members, [])`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ members = ["aaa"] }`),
					EvalExpr(t, "members", `["aaa"]`),
					EvalExpr(t, "members_try", `["aaa"]`),
				),
			},
		},
		{
			name:   "extending global reference with successful tm_try on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("team", `tm_try({ members = ["aaa"] }, {})`),
						),
						Globals(
							Labels("team"),
							Expr("members_ref", "global.team.members"),
							Expr("members_try", `tm_try(global.team.members, [])`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{
						members = ["aaa"]
						members_ref = ["aaa"]
						members_try = ["aaa"]
					}`),
				),
			},
		},
		{
			name:   "undefined references to globals on tm_try",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("a", "value"),
						Expr("b", `tm_try(global.undefined, global.a)`),
						Expr("c", `tm_try(global.a, global.undefined)`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("a", "value"),
					Str("b", "value"),
					Str("c", "value"),
				),
			},
		},
		{
			name:   "global reference with failed tm_try on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "team", `{ members = ["aaa"] }`),
						Expr("members_try", `tm_try(global.team.mistake, [])`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ members = ["aaa"] }`),
					EvalExpr(t, "members_try", "[]"),
				),
			},
		},
		{
			name:   "global interpolating strings",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("str1", "hello"),
						Str("str2", "world"),
						Str("str3", "${global.str1}-${global.str2}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("str1", "hello"),
					Str("str2", "world"),
					Str("str3", "hello-world"),
				),
			},
		},
		{
			// This tests double check that interpolation on a single list
			// produces an actual list object on hcl eval, not a string
			// Which is bizarre...but why not ?
			name:   "global interpolating single list",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `["aaa"]`),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `["aaa"]`),
					EvalExpr(t, "a_interpolated", `["aaa"]`),
				),
			},
		},
		{
			name:   "global interpolating of single number",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Number("a", 1),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Number("a", 1),
					Number("a_interpolated", 1),
				),
			},
		},
		{
			name:   "global interpolating of single boolean",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", true),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Bool("a", true),
					Bool("a_interpolated", true),
				),
			},
		},
		{
			name:   "global interpolating multiple lists fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `["aaa"]`),
						Str("a_interpolated", "${global.a}-${global.a}"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global interpolating list with space fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `["aaa"]`),
						Str("a_interpolated", " ${global.a}"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			// This tests double check that interpolation on a single object/map
			// produces an actual object on hcl eval, not a string.
			// Which is bizarre...but why not ?
			name:   "global interpolating single object",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `{ members = ["aaa"] }`),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{ members = ["aaa"] }`),
					EvalExpr(t, "a_interpolated", `{ members = ["aaa"] }`),
				),
			},
		},
		{
			name:   "global interpolating multiple objects fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `{ members = ["aaa"] }`),
						Str("a_interpolated", "${global.a}-${global.a}"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global interpolating object with space fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "a", `{ members = ["aaa"] }`),
						Str("a_interpolated", "${global.a} "),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global interpolating undefined reference fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("a_interpolated", "${global.undefined}-something"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			// This tests double check that interpolation on a single number
			// produces an actual number on hcl eval, not a string.
			// Which is bizarre...but why not ?
			name:   "global interpolating numbers",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Number("a", 666),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Number("a", 666),
					Number("a_interpolated", 666),
				),
			},
		},
		{
			// Composing numbers on a interpolation works and then produces
			// string. Testing this because this does not work with all types
			// and it is useful for us as maintainers to map/test these different behaviors.
			name:   "global interpolating multiple numbers",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Number("a", 666),
						Str("a_interpolated", "${global.a}-${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Number("a", 666),
					Str("a_interpolated", "666-666"),
				),
			},
		},
		{
			// This tests double check that interpolation on a single boolean
			// produces an actual boolean on hcl eval, not a string.
			// Which is bizarre...but why not ?
			name:   "global interpolating numbers",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", true),
						Str("a_interpolated", "${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Bool("a", true),
					Bool("a_interpolated", true),
				),
			},
		},
		{
			// Composing booleans on a interpolation works and then produces
			// string. Testing this because this does not work with all types
			// and it is useful for us as maintainers to map/test these different behaviors.
			name:   "global interpolating multiple numbers",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", false),
						Str("a_interpolated", "${global.a}-${global.a}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Bool("a", false),
					Str("a_interpolated", "false-false"),
				),
			},
		},
		{
			name:   "global reference with try on root config and value defined on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("team_def", "global.team.def"),
						Expr("team_def_try", `tm_try(global.team.def, {})`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
					EvalExpr(t, "team_def", `{ name = "awesome" }`),
					EvalExpr(t, "team_def_try", `{ name = "awesome" }`),
				),
			},
		},
		{
			name:   "globals cant have blocks inside",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("test", "hallo"),
						Block("notallowed"),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:   "global undefined reference on root",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add:  Globals(Expr("field", "global.unknown")),
				},
				{
					path: "/stack",
					add:  Globals(Str("stack", "whatever")),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global undefined reference on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add:  Globals(Expr("field", "global.unknown")),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global undefined references mixed on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("field_a", "global.unknown"),
						Expr("field_b", "global.unknown_again"),
						Str("valid", "valid"),
						Expr("field_c", "global.oopsie"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global cyclic reference on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "global.b"),
						Expr("b", "global.c"),
						Expr("c", "global.a"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global cyclic references across hierarchy",
			layout: []string{"s:stacks/stack"},
			configs: []hclconfig{
				{
					path: "/",
					add:  Globals(Expr("a", "global.b")),
				},
				{
					path: "/stacks",
					add:  Globals(Expr("b", "global.c")),
				},
				{
					path: "/stacks/stack",
					add:  Globals(Expr("c", "global.a")),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global redefined on different file on stack",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm.hcl",
					add:      Globals(Str("a", "a")),
				},
				{
					path:     "/stack",
					filename: "globals2.tm.hcl",
					add:      Globals(Str("a", "b")),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name: "globals from imported file",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Import(
						EvalExpr(t, "source", `"/other/globals.tm"`),
					),
				},
				{
					path:     "/other",
					filename: "globals.tm",
					add: Globals(
						EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
				),
			},
		},
		{
			name: "globals from imported file and merging",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Doc(
						Import(
							EvalExpr(t, "source", `"/other/globals.tm"`),
						),
						Globals(
							EvalExpr(t, "team2", `"test"`),
						),
					),
				},
				{
					path:     "/other",
					filename: "globals.tm",
					add: Globals(
						EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ def = { name = "awesome" } }`),
					EvalExpr(t, "team2", `"test"`),
				),
			},
		},
		{
			name: "redefined globals from imported file",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "globals.tm",
					add: Doc(
						Import(
							EvalExpr(t, "source", `"/other/globals.tm"`),
						),
						Globals(
							EvalExpr(t, "team", `{ def = { name = "redefined" } }`),
						),
					),
				},
				{
					path:     "/other",
					filename: "globals.tm",
					add: Globals(
						EvalExpr(t, "team", `{ def = { name = "defined" } }`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "team", `{ def = { name = "redefined" } }`),
				),
			},
		},
		{
			name: "globals can reference imported values",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "cfg.tm",
					add: Doc(
						Import(
							EvalExpr(t, "source", `"/other/imported.tm"`),
						),
						Globals(
							Expr("B", `"defined from ${global.A}"`),
						),
					),
				},
				{
					path:     "other",
					filename: "imported.tm",
					add: Globals(
						EvalExpr(t, "A", `"imported"`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "B", `"defined from imported"`),
					EvalExpr(t, "A", `"imported"`),
				),
			},
		},
		{
			name: "imported files are handled before importing file",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "cfg.tm",
					add: Doc(
						Globals(
							Expr("B", `"defined from ${global.A}"`),
						),
						Import(
							EvalExpr(t, "source", `"/other/imported.tm"`),
						),
					),
				},
				{
					path:     "other",
					filename: "imported.tm",
					add: Globals(
						EvalExpr(t, "A", `"other/imported.tm"`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "B", `"defined from other/imported.tm"`),
					EvalExpr(t, "A", `"other/imported.tm"`),
				),
			},
		},
		{
			name: "imported file has redefinition of own imports",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "cfg.tm",
					add: Import(
						EvalExpr(t, "source", `"/other/imported.tm"`),
					),
				},
				{
					path:     "other",
					filename: "imported.tm",
					add: Doc(
						Globals(
							Expr("A", `"defined by other/imported.tm"`),
						),
						Import(
							EvalExpr(t, "source", `"/other2/imported.tm"`),
						),
					),
				},
				{
					path:     "other2",
					filename: "imported.tm",
					add: Globals(
						EvalExpr(t, "A", `"defined by other2/imported.tm"`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "A", `"defined by other/imported.tm"`),
				),
			},
		},
		{
			name: "chained imports references",
			layout: []string{
				"d:other",
				"s:stack",
			},
			configs: []hclconfig{
				{
					path:     "/stack",
					filename: "cfg.tm",
					add: Doc(
						Globals(
							Expr("A", "global.B"),
						),
						Import(
							EvalExpr(t, "source", `"/other/imported.tm"`),
						),
					),
				},
				{
					path:     "other",
					filename: "imported.tm",
					add: Doc(
						Globals(
							Expr("B", "global.C"),
						),
						Import(
							EvalExpr(t, "source", `"/other2/imported.tm"`),
						),
					),
				},
				{
					path:     "other2",
					filename: "imported.tm",
					add: Globals(
						EvalExpr(t, "C", `"defined at other2/imported.tm"`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "A", `"defined at other2/imported.tm"`),
					EvalExpr(t, "B", `"defined at other2/imported.tm"`),
					EvalExpr(t, "C", `"defined at other2/imported.tm"`),
				),
			},
		},
		{
			name: "unset globals",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"s:stacks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("a", "a"),
						Str("b", "b"),
						Str("c", "c"),
					),
				},
				{
					path: "/stacks",
					add: Globals(
						Str("d", "d"),
						Expr("b", "unset"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Globals(
						Expr("c", "unset"),
						Expr("d", "unset"),
					),
				},
				{
					path: "/stacks/stack-3",
					add: Globals(
						Str("b", "redefined"),
					),
				},
				{
					path: "/stacks/stack-4",
					add: Globals(
						Expr("a", "unset"),
						Expr("c", "unset"),
						Expr("d", "unset"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("a", "a"),
				),
				"/stacks/stack-2": Globals(
					Str("a", "a"),
					Str("c", "c"),
					Str("d", "d"),
				),
				"/stacks/stack-3": Globals(
					Str("a", "a"),
					Str("b", "redefined"),
					Str("c", "c"),
					Str("d", "d"),
				),
			},
		},
		{
			name:   "operating two unset fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "unset + unset"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "operating unset and other type fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "unset + 666"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "interpolating unset fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("a", "${unset}"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "unset on list fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "[unset]"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "unset on obj fails",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "{ a = unset }"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "global with tm_ternary returning literals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", true),
						Expr("val", "tm_ternary(global.a, 1, 2)"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Bool("a", true),
					Number("val", 1),
				),
			},
		},
		{
			name:   "global with tm_ternary with different branch types",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", true),
						Expr("val", "tm_ternary(!global.a, [], {})"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Bool("a", true),
					EvalExpr(t, "val", `{}`),
				),
			},
		},
		{
			name:   "global with tm_ternary returning partials",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Bool("a", true),
						Expr("val", "tm_ternary(global.a, [local.a], 2)"),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)
			projmeta := s.LoadProjectMetadata()

			for _, globalBlock := range tcase.configs {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				filename := config.DefaultFilename
				if globalBlock.filename != "" {
					filename = globalBlock.filename
				}
				test.AppendFile(t, path, filename, globalBlock.add.String())
			}

			wantGlobals := tcase.want

			cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
			if err != nil {
				errtest.Assert(t, err, tcase.wantErr)
				return
			}

			stackEntries, err := terramate.ListStacks(cfg)
			assert.NoError(t, err)

			var stacks stack.List
			for _, entry := range stackEntries {
				st := entry.Stack
				stacks = append(stacks, st)

				t.Logf("loading globals for stack: %s", st.Path())

				gotReport := stack.LoadStackGlobals(s.Config(), projmeta, st)
				errtest.Assert(t, gotReport.AsError(), tcase.wantErr)
				if tcase.wantErr != nil {
					continue
				}

				want, ok := wantGlobals[st.Path().String()]
				if !ok {
					want = Globals()
				}
				delete(wantGlobals, st.Path().String())

				// Could have one type for globals configs and another type
				// for wanted evaluated globals, but that would make
				// globals building more annoying (two sets of functions).
				if want.HasExpressions() {
					t.Fatal("wanted globals definition contains expressions, they should be defined only by evaluated values")
					t.Errorf("wanted globals definition:\n%s\n", want)
				}

				got := gotReport.Globals
				gotAttrs := got.AsValueMap()
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
			}

			if len(wantGlobals) > 0 {
				t.Fatalf("wanted stack globals: %v that was not found on stacks: %v", wantGlobals, stacks)
			}
		})
	}
}

func TestLoadGlobalsErrors(t *testing.T) {
	type (
		cfg struct {
			path string
			body string
		}
		testcase struct {
			name    string
			layout  []string
			configs []cfg
			want    error
		}
	)

	// These test scenarios where quite hard to describe with the
	// core test fixture (core model doesn't allow duplicated fields
	// by nature, and it never creates malformed global blocks),
	// hence this separate error tests exists :-).

	tcases := []testcase{
		{
			name:   "stack config has invalid global definition",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/stack",
					body: `
					  globals {
					    a = "hi"
					`,
				},
			},
			want: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name:   "root config has invalid global definition",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/",
					body: `
					  globals {
					    a = "hi"
					`,
				},
			},
			want: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name:   "stack config has global redefinition on single block",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/stack",
					body: `
					  globals {
					    a = "hi"
					    a = 5
					  }
					`,
				},
			},
			// FIXME(katcipis): would be better to have ErrGlobalRedefined
			// for now we get an error directly from hcl for this.
			want: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name:   "root config has global redefinition on single block",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/",
					body: `
					  globals {
					    a = "hi"
					    a = 5
					  }
					`,
				},
			},
			// FIXME(katcipis): would be better to have ErrGlobalRedefined
			// for now we get an error directly from hcl for this.
			want: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name:   "stack config has global redefinition on multiple blocks",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/stack",
					body: `
					  globals {
					    a = "hi"
					  }
					  globals {
					    a = 5
					  }
					  globals {
					    a = true
					  }
					`,
				},
			},
			want: errors.E(globals.ErrRedefined),
		},
		{
			name:   "root config has global redefinition on multiple blocks",
			layout: []string{"s:stack"},
			configs: []cfg{
				{
					path: "/",
					body: `
					  globals {
					    a = "hi"
					  }
					  globals {
					    a = 5
					  }
					`,
				},
			},
			want: errors.E(globals.ErrRedefined),
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, c := range tcase.configs {
				path := filepath.Join(s.RootDir(), c.path)
				test.AppendFile(t, path, config.DefaultFilename, c.body)
			}

			cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
			// TODO(i4k): this better not be tested here.
			if errors.IsKind(tcase.want, hcl.ErrHCLSyntax) {
				errtest.Assert(t, err, tcase.want)
			}

			if err != nil {
				return
			}

			stacks, err := stack.LoadAll(cfg)
			assert.NoError(t, err)
			projmeta := stack.NewProjectMetadata(s.RootDir(), stacks)

			for _, st := range stacks {
				report := stack.LoadStackGlobals(s.Config(), projmeta, st)
				errtest.Assert(t, report.AsError(), tcase.want)
			}
		})
	}
}

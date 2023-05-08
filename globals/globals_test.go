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

package globals_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog"

	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"

	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/zclconf/go-cty-debug/ctydebug"
)

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

// TODO(katcipis): add tests related to tf functions that depend on filesystem
// (BaseDir parameter passed on Scope when creating eval context).
func TestLoadGlobals(t *testing.T) {
	t.Parallel()

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
			name: "multiple stacks with config on parent dir extended by children",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{path: "/stacks", add: Globals(Expr("parent", "global.a"))},
				{path: "/stacks/stack-1", add: Globals(Number("a", 1))},
				{path: "/stacks/stack-2", add: Globals(Number("a", 2))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("parent", 1),
					Number("a", 1),
				),
				"/stacks/stack-2": Globals(
					Number("parent", 2),
					Number("a", 2),
				),
			},
		},
		{
			name: "extending global but referencing by indexing the globals map",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{path: "/stacks", add: Globals(Expr("parent", `global["a"]`))},
				{path: "/stacks/stack-1", add: Globals(Number("a", 1))},
				{path: "/stacks/stack-2", add: Globals(Number("a", 2))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("parent", 1),
					Number("a", 1),
				),
				"/stacks/stack-2": Globals(
					Number("parent", 2),
					Number("a", 2),
				),
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
				`s:stacks/stack-2:id=stack-2-id;description=someDescriptionStack2;tags=["tag1", "tag2", "tag3"]`,
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
						Expr("stack_tags", "terramate.stack.tags"),
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
						Expr("stack_tags", "terramate.stack.tags"),
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
					EvalExpr(t, "stack_tags", "tolist([])"),
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
					EvalExpr(t, "stack_tags", `tolist(["tag1", "tag2", "tag3"])`),
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
			name:   "stack with globals referencing globals by indexing",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Str("field", "some-string"),
						Expr("stack_path", "terramate.stack.path.absolute"),
						Expr("ref_field", `global["field"]`),
						Expr("ref_stack_path", `global["stack_path"]`),
						Expr("interpolation", `"${global["ref_stack_path"]}-${global["ref_field"]}"`),
						Expr("ref_interpolation", `global["interpolation"]`),
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
			name:   "globals referencing globals by nested indexing",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("object"),
							Str("dummy", "test"),
						),
						Globals(
							Expr("terramate", `tm_upper(global.object["child"]["string"])`),
						),
						Globals(
							Labels("object", "child"),
							Str("string", "terramate"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("terramate", "TERRAMATE"),
					EvalExpr(t, "object", `{
						child = {
							string = "terramate"
						},
						dummy = "test"
					}`),
				),
			},
		},
		{
			name:   "global referencing traverse-index",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "global.b[0]"),
						Expr("b", "[1]"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Number("a", 1),
					EvalExpr(t, "b", "[1]"),
				),
			},
		},
		{
			name:   "global referencing traverse-splat",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", "global.b.*"),
						Expr("b", "{c = 1}"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `[{c = 1}]`),
					EvalExpr(t, "b", "{c = 1}"),
				),
			},
		},
		{
			name:   "import merge with pending variable",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path:     "/modules",
					filename: "file.tm",
					add: Doc(
						Globals(
							Labels("obj"),
							Expr("a", `{
							b = 1
							c = global.pending
						}`),
						),
						Import(
							Str("source", "/modules/nested/file.tm"),
						),
					),
				},
				{
					path:     "/modules/nested",
					filename: "file.tm",
					add: Globals(
						Labels("obj", "a"),
						Number("nested", 1),
						Expr("nested2", "global.other_pending"),
					),
				},
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj", "a"),
							Number("b", 2),
						),
						Globals(
							Number("pending", 1),
							Number("other_pending", 2),
						),
						Import(
							Str("source", "/modules/file.tm"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Number("pending", 1),
					Number("other_pending", 2),
					EvalExpr(t, "obj", `{
						a = {
							b = 2
							c = 1
							nested = 1
							nested2 = 2
						}
					}`),
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
			// This is a regression test for a severe bug on extension
			name: "multiple stacks extending imported globals on parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/modules",
					filename: "config.tm",
					add: Globals(
						Labels("label"),
						EvalExpr(t, "obj", `{
							data = 1,
						}`),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Import(
							Str("source", "/modules/config.tm"),
						),
						Globals(
							Str("data", "parent"),
						),
					),
				},
				{
					path:     "/stacks",
					filename: "config.tm",
					add: Globals(
						Labels("label", "obj"),
						Number("ext_data", 777),
						Number("data", 666),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Str("data", "parent"),
					EvalExpr(t, "label", `{
					  obj = {
					    data = 666,
					    ext_data = 777,
					  }
					}`),
				),
				"/stacks/stack-2": Globals(
					Str("data", "parent"),
					EvalExpr(t, "label", `{
					  obj = {
					    data = 666,
					    ext_data = 777,
					  }
					}`),
				),
			},
		},
		{
			name: "extending global with pending set object",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/modules",
					filename: "config.tm",
					add: Globals(
						Labels("label"),
						Bool("enabled", true),
						Str("source", "hashicorp/google"),
						Expr("obj", `{
							data1 = tm_try(global.pending, 667)
							data2 = tm_try(global.not_pending, 668)
						}`),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Globals(
							Labels("label", "obj"),
							Number("data1", 3),
						),
						Import(
							Str("source", "/modules/config.tm"),
						),
						Globals(
							Number("pending", 666),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("pending", 666),
					EvalExpr(t, "label", `{
					  enabled = true
  					  source  = "hashicorp/google"
					  obj = {
					    data1 = 3
						data2 = 668
					  }
					}`),
				),
				"/stacks/stack-2": Globals(
					Number("pending", 666),
					EvalExpr(t, "label", `{
						enabled = true
  					    source  = "hashicorp/google"
						obj = {
						  data1 = 3
						  data2 = 668
						}
					  }`),
				),
			},
		},
		{
			name: "extending complex global with pending set object",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/modules",
					filename: "config.tm",
					add: Doc(
						Globals(
							Str("a", "test"),
							Number("b", 1),
						),
						Globals(
							Labels("label"),
							Bool("enabled", true),
							Str("source", "hashicorp/google"),
							Expr("obj", `{
									data1 = tm_try(global.pending, 667)
									data2 = tm_try(global.not_pending, 668)
								}`),
						),
						Globals(
							Labels("label", "obj"),
							Bool("enabled", true),
							Str("source", "hashicorp/google"),
						),
						Globals(
							Str("c", "test"),
							Number("d", 1),
						),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Globals(
							Labels("label", "obj"),
							Number("data1", 3),
						),
						Import(
							Str("source", "/modules/config.tm"),
						),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "config.tm",
					add: Doc(
						Globals(
							Number("pending", 666),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("pending", 666),
					Str("a", "test"),
					Number("b", 1),
					Str("c", "test"),
					Number("d", 1),
					EvalExpr(t, "label", `{
							  enabled = true
		  					  source  = "hashicorp/google"
							  obj = {
							    data1 = 3
								data2 = 668
								enabled = true
		  					  source  = "hashicorp/google"
							  }
							}`),
				),
				"/stacks/stack-2": Globals(
					Str("a", "test"),
					Number("b", 1),
					Str("c", "test"),
					Number("d", 1),
					EvalExpr(t, "label", `{
								enabled = true
		  					    source  = "hashicorp/google"
								obj = {
								  data1 = 3
								  data2 = 668
								  enabled = true
		  					  	  source  = "hashicorp/google"
								}
							  }`),
				),
			},
		},
		{
			name: "imports from different cfgdir extending same object",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/modules/a",
					filename: "file1.tm",
					add: Doc(
						Globals(
							Labels("label"),
							Expr("obj", `{
									data1 = tm_try(global.pending, 1)
									data2 = tm_try(global.not_pending, 2)
								}`),
						),
					),
				},
				{
					path:     "/modules/b",
					filename: "file2.tm",
					add: Doc(
						Globals(
							Labels("label"),
							Expr("obj", `{
									data1 = tm_try(global.pending, 3)
									data2 = tm_try(global.not_pending, 4)
								}`),
						),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Import(
							Str("source", "/modules/a/file1.tm"),
						),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "config.tm",
					add: Doc(
						Globals(
							Number("pending", 666),
						),
						Import(
							Str("source", "/modules/b/file2.tm"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("pending", 666),
					EvalExpr(t, "label", `{
							  obj = {
							    data1 = 666
								data2 = 4
							  }
							}`),
				),
				"/stacks/stack-2": Globals(
					EvalExpr(t, "label", `{
								obj = {
								  data1 = 1
								  data2 = 2
								}
							  }`),
				),
			},
		},
		{
			name: "extending complex global with pending set object without imports",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Globals(
							Str("a", "test"),
							Number("b", 1),
						),
						Globals(
							Labels("label"),
							Bool("enabled", true),
							Str("source", "hashicorp/google"),
							Expr("obj", `{
										data1 = tm_try(global.pending, 667)
										data2 = tm_try(global.not_pending, 668)
									}`),
						),
						Globals(
							Labels("label", "obj"),
							Bool("enabled", true),
							Str("source", "hashicorp/google"),
						),
						Globals(
							Str("c", "test"),
							Number("d", 1),
						),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "config.tm",
					add: Doc(
						Globals(
							Number("pending", 666),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": Globals(
					Number("pending", 666),
					Str("a", "test"),
					Number("b", 1),
					Str("c", "test"),
					Number("d", 1),
					EvalExpr(t, "label", `{
							  enabled = true
		  					  source  = "hashicorp/google"
							  obj = {
							    data1 = 666
								data2 = 668
								enabled = true
		  					  source  = "hashicorp/google"
							  }
							}`),
				),
				"/stacks/stack-2": Globals(
					Str("a", "test"),
					Number("b", 1),
					Str("c", "test"),
					Number("d", 1),
					EvalExpr(t, "label", `{
								enabled = true
		  					    source  = "hashicorp/google"
								obj = {
								  data1 = 667
								  data2 = 668
								  enabled = true
		  					  	  source  = "hashicorp/google"
								}
							  }`),
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
			name: "extending globals set on same level",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("obj", `{
								a = {
									z = tm_try(global.d, false)
								}
							}`),
						),
						Globals(
							Labels("obj", "a"),
							Expr("b", "{}"),
						),
						Globals(
							Labels("obj", "a", "b"),
							Number("number", 1),
						),
						Globals(
							Str("d", "test"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{
						a = {
							b = {
								number = 1
							}
							z = "test"
						}
					}`),
					Str("d", "test"),
				),
			},
		},
		{
			name: "extending globals with empty block",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj"),
							Expr("a", `[1, 2, 3]`),
							Number("b", 1),
						),
						Globals(
							Labels("obj"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{
						a = [1, 2, 3]
						b = 1
					}`),
				),
			},
		},
		{
			name: "extending globals with empty block in child directory",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Labels("obj"),
						Expr("a", `[1, 2, 3]`),
						Number("b", 1),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("obj"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{
						a = [1, 2, 3]
						b = 1
					}`),
				),
			},
		},
		{
			name: "extending globals multiple times with empty block",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj"),
						),
						Globals(
							Labels("obj"),
							EvalExpr(t, "iam", `[1, 2, 3]`),
						),
						Globals(
							Labels("obj"),
						),
						Globals(
							Labels("obj"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{ iam = [1, 2, 3] }`),
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
			name: "inheriting labeled globals without attributes",
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
			name: "extending non-existent global with a non-identifier first label - fails",
			layout: []string{
				"s:stacks/stack-a",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-a",
					add: Doc(
						Globals(
							Labels("-this-is-not.a.*valid%identifier"),
							Number("number", 1),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name: "internal global keys can be extended with non-identifier label",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("obj", ".this.is.*not*a/valid/identifier$"),
							Str("string", "test"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "obj", `{
						".this.is.*not*a/valid/identifier$" = {
							string = "test"
						}
					}`),
				),
			},
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
			name: "extending parent list fails",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `[]`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("lst"),
						Expr("other", `[]`),
					),
				},
			},
			wantErr: errors.E(eval.ErrCannotExtendObject),
		},
		{
			name: "extending list from object fails",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("test"),
							Expr("lst", `[1, 2, 3]`),
						),
						Globals(
							Labels("test", "lst"),
							Expr("values", `[1, 2]`),
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
			name:   "tm_hcl_expression is not available on globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("data", `tm_hcl_expression("test")`),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "tm_vendor is not available on globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("data", `tm_vendor("github.com/mineiros-io/terramate?ref=test")`),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
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
			name:   "unset on extended global",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("a", `{
							a = "must be unset"
						}`),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Labels("a"),
						Expr("a", "unset"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{}`),
				),
			},
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
		{
			name:   "tm_try with only root traversal",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Expr("val", `tm_try(global, "default")`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "val", `{}`),
				),
			},
		},
		{
			name:   "globals.map label conflicts with global name",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("name", "test"),
						Map(
							Labels("name"),
							Expr("for_each", "[]"),
							Str("key", "a"),
							Str("value", "a"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrRedefined),
		},
		{
			name:   "invalid globals.map key",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "something"), // keyword, not a string
							Str("value", "else"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "invalid globals.map value",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Str("key", "something"),
							Expr("value", "else"), // keyword, not an expression
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrEval),
		},
		{
			name:   "simple globals.map without using element",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Str("key", "something"),
							Str("value", "else"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						something = "else"
					}`),
				),
			},
		},
		{
			name:   "conflicting globals.map name with other globals",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Str("var", "test"),
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			wantErr: errors.E(globals.ErrRedefined),
		},
		{
			name:   "simple globals.map ",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "multiple globals.map blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
						Map(
							Labels("var2"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "element.new"),
							Expr("value", "element.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
					EvalExpr(t, "var2", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "simple globals.map with different iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `["a", "b", "c"]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "var", `{
						a = "a"
						b = "b"
						c = "c"
					}`),
				),
			},
		},
		{
			name:   "globals.map unknowns are postponed in the evaluator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `[global.val1, global.val2, global.val3]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
						Str("val2", "val2"),
					),
				},
				{
					path: "/",
					add: Globals(
						Str("val1", "val1"),
						Str("val3", "val3"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("val1", "val1"),
					Str("val2", "val2"),
					Str("val3", "val3"),
					EvalExpr(t, "var", `{
						val1 = "val1"
						val2 = "val2"
						val3 = "val3"
					}`),
				),
			},
		},
		{
			name:   "indexing references are postponed until all other globals with base prefix are evaluated",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("use_providers", `{}`),
							Expr("_available_providers", `{
								aws = {
								  source  = "hashicorp/aws"
								  version = "~> 4.14"
								}
								vault = {
								  source  = "hashicorp/vault"
								  version = "~> 3.10"
								}
								postgresql = {
								  source  = "cyrilgdn/postgresql"
								  version = "~> 1.18.0"
								}
								mysql = {
								  source  = "petoju/mysql"
								  version = "~> 3.0.29"
								}
							  }`),
							Expr("required_providers", `{for k, v in global._available_providers : k => v if tm_try(global.use_providers[k], false)}`),
						),
						Globals(
							Labels("use_providers"),
							Bool("aws", true),
						),
						Globals(
							Labels("use_providers"),
							Bool("mysql", true),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "use_providers", `{
						aws = true
						mysql = true	
					}`),
					EvalExpr(t, "_available_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						vault = {
						  source  = "hashicorp/vault"
						  version = "~> 3.10"
						}
						postgresql = {
						  source  = "cyrilgdn/postgresql"
						  version = "~> 1.18.0"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
					EvalExpr(t, "required_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
				),
			},
		},
		{
			name:   "indexing references are postponed until all other globals with base prefix are evaluated - case 2",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("use", `{
								providers = {}
							}`),
							Expr("_available_providers", `{
								aws = {
								  source  = "hashicorp/aws"
								  version = "~> 4.14"
								}
								vault = {
								  source  = "hashicorp/vault"
								  version = "~> 3.10"
								}
								postgresql = {
								  source  = "cyrilgdn/postgresql"
								  version = "~> 1.18.0"
								}
								mysql = {
								  source  = "petoju/mysql"
								  version = "~> 3.0.29"
								}
							  }`),
							Expr("required_providers", `{for k, v in global._available_providers : k => v if tm_try(global.use.providers[k], false)}`),
						),
						Globals(
							Labels("use", "providers"),
							Bool("aws", true),
						),
						Globals(
							Labels("use", "providers"),
							Bool("mysql", true),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "use", `{
						providers = {
							aws = true
							mysql = true
						}	
					}`),
					EvalExpr(t, "_available_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						vault = {
						  source  = "hashicorp/vault"
						  version = "~> 3.10"
						}
						postgresql = {
						  source  = "cyrilgdn/postgresql"
						  version = "~> 1.18.0"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
					EvalExpr(t, "required_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
				),
			},
		},
		{
			name:   "indexing references are postponed until all other globals with base prefix are evaluated - case 3",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Labels("a", "b", "c"),
							Expr("providers", `{}`),
							Expr("_available_providers", `{
								aws = {
								  source  = "hashicorp/aws"
								  version = "~> 4.14"
								}
								vault = {
								  source  = "hashicorp/vault"
								  version = "~> 3.10"
								}
								postgresql = {
								  source  = "cyrilgdn/postgresql"
								  version = "~> 1.18.0"
								}
								mysql = {
								  source  = "petoju/mysql"
								  version = "~> 3.0.29"
								}
							  }`),
						),
						Globals(
							Labels("a", "b", "c"),
							Expr("required_providers", `{for k, v in global.a.b.c._available_providers : k => v if tm_try(global.a.b.c.providers[k], false)}`),
						),
						Globals(
							Labels("a", "b", "c", "providers"),
							Bool("aws", true),
						),
						Globals(
							Labels("a", "b", "c", "providers"),
							Bool("mysql", true),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "a", `{
						b = {
							c = {
								_available_providers = {
									aws = {
										source  = "hashicorp/aws"
										version = "~> 4.14"
									}
									vault = {
										source  = "hashicorp/vault"
										version = "~> 3.10"
									}
									postgresql = {
										source  = "cyrilgdn/postgresql"
										version = "~> 1.18.0"
									}
									mysql = {
										source  = "petoju/mysql"
										version = "~> 3.0.29"
									}
								}
								providers = {
									aws = true
									mysql = true
								}
								required_providers = {
									aws = {
									  source  = "hashicorp/aws"
									  version = "~> 4.14"
									}
									mysql = {
									  source  = "petoju/mysql"
									  version = "~> 3.0.29"
									}
								  }
							}
						}
					}`),
				),
			},
		},
		{
			name:   "expression dependency are tracked when using indexing",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						Globals(
							Expr("use_providers", `{}`),
							Expr("_available_providers", `{
								aws = {
								  source  = "hashicorp/aws"
								  version = "~> 4.14"
								}
								vault = {
								  source  = "hashicorp/vault"
								  version = "~> 3.10"
								}
								postgresql = {
								  source  = "cyrilgdn/postgresql"
								  version = "~> 1.18.0"
								}
								mysql = {
								  source  = "petoju/mysql"
								  version = "~> 3.0.29"
								}
							  }`),
							Expr("required_providers", `{for k, v in global._available_providers : k => v if tm_try(global["use_providers"][k], false)}`),
						),
						Globals(
							Labels("use_providers"),
							Bool("aws", true),
						),
						Globals(
							Labels("use_providers"),
							Bool("mysql", true),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "use_providers", `{
						aws = true
						mysql = true	
					}`),
					EvalExpr(t, "_available_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						vault = {
						  source  = "hashicorp/vault"
						  version = "~> 3.10"
						}
						postgresql = {
						  source  = "cyrilgdn/postgresql"
						  version = "~> 1.18.0"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
					EvalExpr(t, "required_providers", `{
						aws = {
						  source  = "hashicorp/aws"
						  version = "~> 4.14"
						}
						mysql = {
						  source  = "petoju/mysql"
						  version = "~> 3.0.29"
						}
					  }`),
				),
			},
		},
		{
			name:   "globals.map unknowns are postponed in the evaluator even when parent depends on child",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("var"),
							Expr("iterator", "el"),
							Expr("for_each", `[global.val1, global.val2, global.val3]`),
							Expr("key", "el.new"),
							Expr("value", "el.new"),
						),
						Str("val2", "val2"),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Str("val1", "val1"),
						Str("val3", "val3"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					Str("val1", "val1"),
					Str("val2", "val2"),
					Str("val3", "val3"),
					EvalExpr(t, "var", `{
						val1 = "val1"
						val2 = "val2"
						val3 = "val3"
					}`),
				),
			},
		},
		{
			name:   "element.old is undefined on first iteration of a given key",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Expr("value", `tm_try(element.old, 0) + 1`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = 1
  						tiago  = 2
  						soeren = 1
					}`),
				),
			},
		},
		{
			name:   "using element.old in value attr to count people",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Expr("value", `{
								count = tm_try(element.old.count, 0) + 1	
							}`),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = {count = 1}
  						tiago  = {count = 2}
  						soeren = {count = 1}
					}`),
				),
			},
		},
		{
			name:   "using element.old in value block to count people",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Map(
							Labels("people_count"),
							Expr("for_each", `["marius", "tiago", "soeren", "tiago"]`),
							Expr("key", "element.new"),
							Value(
								Expr("count", `tm_try(element.old.count, 0) + 1`),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "people_count", `{
						marius = {count = 1}
  						tiago  = {count = 2}
  						soeren = {count = 1}
					}`),
				),
			},
		},
		{
			name:   "globals.map is recursive",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `["a", "b", "c"]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `["a", "b", "c"]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						b = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						c = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map with multiple map blocks",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `["a", "b", "c"]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "global.lst"),
									Expr("key", "element.new"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `["a", "b", "c"]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						b = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
						c = {
							some = "value"
							var = {
								a = "a"
								b = "b"
								c = "c"
							}
							var2 = {
								a = "a"
								b = "b"
								c = "c"
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map reusing element iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `[
							{
								val: "a", 
								lst: [1, 2, 3]
							},
							{
								val: "b",
								lst: [4, 5, 6]
							},
							{
								val: "c",
								lst: [7, 8, 9]
							}
						]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new.val"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(element.new)"),
									Expr("value", "element.new"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(element.new)"),
									Expr("value", "element.new"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `[
						{
							val: "a", 
							lst: [1, 2, 3]
						},
						{
							val: "b",
							lst: [4, 5, 6]
						},
						{
							val: "c",
							lst: [7, 8, 9]
						}
					]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
							var2 = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
						}
						b = {
							some = "value"
							var = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
							var2 = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
						}
						c = {
							some = "value"
							var = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
							var2 = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
						}
					}`),
				),
			},
		},
		{
			name:   "recursive globals.map reusing with different nested iterator",
			layout: []string{"s:stack"},
			configs: []hclconfig{
				{
					path: "/",
					add: Globals(
						Expr("lst", `[
							{
								val: "a", 
								lst: [1, 2, 3]
							},
							{
								val: "b",
								lst: [4, 5, 6]
							},
							{
								val: "c",
								lst: [7, 8, 9]
							}
						]`),
						Map(
							Labels("var"),
							Expr("for_each", `global.lst`),
							Expr("key", "element.new.val"),

							Value(
								Str("some", "value"),
								Map(
									Labels("var"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(el.new)"),
									Expr("value", "el.new"),
									Expr("iterator", "el"),
								),
								Map(
									Labels("var2"),
									Expr("for_each", "element.new.lst"),
									Expr("key", "tm_tostring(el2.new)"),
									Expr("value", "el2.new"),
									Expr("iterator", "el2"),
								),
							),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "lst", `[
						{
							val: "a", 
							lst: [1, 2, 3]
						},
						{
							val: "b",
							lst: [4, 5, 6]
						},
						{
							val: "c",
							lst: [7, 8, 9]
						}
					]`),
					EvalExpr(t, "var", `{
						a = {
							some = "value"
							var = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
							var2 = {
								"1" = 1
								"2" = 2
								"3" = 3
							}
						}
						b = {
							some = "value"
							var = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
							var2 = {
								"4" = 4
								"5" = 5
								"6" = 6
							}
						}
						c = {
							some = "value"
							var = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
							var2 = {
								"7" = 7
								"8" = 8
								"9" = 9
							}
						}
					}`),
				),
			},
		},
		{

			name: "regression test for a bug which incorrectly returned ErrRedefined errors",
			layout: []string{
				"s:stack",
				"d:modules",
			},
			configs: []hclconfig{
				{
					path:     "/modules",
					filename: "imported.tm",
					add: Doc(
						Globals(
							Labels("hello"),
							Expr("world", `{
								a = 1
							}`),
						),
					),
				},
				{
					path: "/",
					add: Doc(
						Import(
							Str("source", `/modules/imported.tm`),
						),
						Globals(
							Labels("hello", "world"),
							Number("a", 2),
						),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": Globals(
					EvalExpr(t, "hello", `{
							world = {
								a = 2
							}
						}`),
				),
			},
		},
	}

	for _, tcase := range tcases {
		testGlobals(t, tcase)
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

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

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

			stacks, err := config.LoadAllStacks(cfg)
			assert.NoError(t, err)
			for _, elem := range stacks {
				report := globals.ForStack(s.Config(), elem.Stack)
				errtest.Assert(t, report.AsError(), tcase.want)
			}
		})
	}
}

func testGlobals(t *testing.T, tcase testcase) {
	t.Run(tcase.name, func(t *testing.T) {
		s := sandbox.New(t)
		s.BuildTree(tcase.layout)
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

		stackEntries, err := stack.List(cfg)
		assert.NoError(t, err)

		var stacks config.List[*config.SortableStack]
		for _, entry := range stackEntries {
			st := entry.Stack
			stacks = append(stacks, st.Sortable())

			t.Logf("loading globals for stack: %s", st.Dir)

			gotReport := globals.ForStack(s.Config(), st)
			errtest.Assert(t, gotReport.AsError(), tcase.wantErr)
			if tcase.wantErr != nil {
				continue
			}

			want, ok := wantGlobals[st.Dir.String()]
			if !ok {
				want = Globals()
			}
			delete(wantGlobals, st.Dir.String())

			// Could have one type for globals configs and another type
			// for wanted evaluated globals, but that would make
			// globals building more annoying (two sets of functions).
			if want.HasExpressions() {
				t.Errorf("wanted globals definition contains expressions, they should be defined only by evaluated values")
				t.Fatalf("wanted globals definition:\n%s\n", want)
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

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

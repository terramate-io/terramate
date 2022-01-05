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

package terramate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/zclconf/go-cty-debug/ctydebug"
)

// TODO(katcipis): add tests related to tf functions that depend on filesystem
// (BaseDir parameter passed on Scope when creating eval context).

func TestLoadGlobals(t *testing.T) {
	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name    string
			layout  []string
			globals []globalsBlock
			want    map[string]*hclwrite.Block
			wantErr bool
		}
	)

	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	expr := hclwrite.Expression
	attr := func(name, expr string) hclwrite.BlockBuilder {
		return hclwrite.AttributeValue(t, name, expr)
	}
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

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
			name:   "single stack with its own globals",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					str("some_string", "string"),
					number("some_number", 777),
					boolean("some_bool", true),
				),
			},
		},
		{
			name:   "single stack with three globals blocks",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{path: "/stack", add: globals(str("str", "hi"))},
				{path: "/stack", add: globals(number("num", 666))},
				{path: "/stack", add: globals(boolean("bool", false))},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					str("str", "hi"),
					number("num", 666),
					boolean("bool", false),
				),
			},
		},
		{
			name: "multiple stacks with config on parent dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{path: "/stacks", add: globals(str("parent", "hi"))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(str("parent", "hi")),
				"/stacks/stack-2": globals(str("parent", "hi")),
			},
		},
		{
			name: "multiple stacks with config on root dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{path: "/", add: globals(str("root", "hi"))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(str("root", "hi")),
				"/stacks/stack-2": globals(str("root", "hi")),
			},
		},
		{
			name: "multiple stacks merging no overriding",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{path: "/", add: globals(str("root", "root"))},
				{path: "/stacks", add: globals(boolean("parent", true))},
				{path: "/stacks/stack-1", add: globals(number("stack", 666))},
				{path: "/stacks/stack-2", add: globals(number("stack", 777))},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(
					str("root", "root"),
					boolean("parent", true),
					number("stack", 666),
				),
				"/stacks/stack-2": globals(
					str("root", "root"),
					boolean("parent", true),
					number("stack", 777),
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
			globals: []globalsBlock{
				{
					path: "/",
					add: globals(
						str("field_a", "field_a_root"),
						str("field_b", "field_b_root"),
					),
				},
				{
					path: "/stacks",
					add: globals(
						str("field_b", "field_b_stacks"),
						str("field_c", "field_c_stacks"),
						str("field_d", "field_d_stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: globals(
						str("field_a", "field_a_stack_1"),
						str("field_b", "field_b_stack_1"),
						str("field_c", "field_c_stack_1"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: globals(
						str("field_d", "field_d_stack_2"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(
					str("field_a", "field_a_stack_1"),
					str("field_b", "field_b_stack_1"),
					str("field_c", "field_c_stack_1"),
					str("field_d", "field_d_stacks"),
				),
				"/stacks/stack-2": globals(
					str("field_a", "field_a_root"),
					str("field_b", "field_b_stacks"),
					str("field_c", "field_c_stacks"),
					str("field_d", "field_d_stack_2"),
				),
				"/stacks/stack-3": globals(
					str("field_a", "field_a_root"),
					str("field_b", "field_b_stacks"),
					str("field_c", "field_c_stacks"),
					str("field_d", "field_d_stacks"),
				),
			},
		},
		{
			name: "stacks referencing metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{
					path: "/stacks/stack-1",
					add: globals(
						expr("stack_path", "terramate.path"),
						expr("interpolated", `"prefix-${terramate.name}-suffix"`),
					),
				},
				{
					path: "/stacks/stack-2",
					add:  globals(expr("stack_path", "terramate.path")),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(
					str("stack_path", "/stacks/stack-1"),
					str("interpolated", "prefix-stack-1-suffix"),
				),
				"/stacks/stack-2": globals(str("stack_path", "/stacks/stack-2")),
			},
		},
		{
			name: "stacks using functions and metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{
					path: "/stacks/stack-1",
					add: globals(
						expr("interpolated", `"prefix-${replace(terramate.path, "/", "@")}-suffix"`),
					),
				},
				{
					path: "/stacks/stack-2",
					add: globals(
						expr("stack_path", `replace(terramate.path, "/", "-")`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(
					str("interpolated", "prefix-@stacks@stack-1-suffix"),
				),
				"/stacks/stack-2": globals(str("stack_path", "-stacks-stack-2")),
			},
		},
		{
			name:   "stack with globals referencing globals",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						str("field", "some-string"),
						expr("stack_path", "terramate.path"),
						expr("ref_field", "global.field"),
						expr("ref_stack_path", "global.stack_path"),
						expr("interpolation", `"${global.ref_stack_path}-${global.ref_field}"`),
						expr("ref_interpolation", "global.interpolation"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					str("field", "some-string"),
					str("stack_path", "/stack"),
					str("ref_field", "some-string"),
					str("ref_stack_path", "/stack"),
					str("interpolation", "/stack-some-string"),
					str("ref_interpolation", "/stack-some-string"),
				),
			},
		},
		{
			name:   "stack with globals referencing globals hierarchically no overriding",
			layout: []string{"s:envs/prod/stacks/stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add: globals(
						str("root_field", "root-data"),
						number("root_number", 666),
						boolean("root_bool", true),
						expr("root_stack_ref", "global.stack_inter"),
					),
				},
				{
					path: "/envs",
					add: globals(
						expr("env_metadata", "terramate.path"),
						expr("env_root_ref", "global.root_field"),
					),
				},
				{
					path: "/envs/prod",
					add:  globals(str("env", "prod")),
				},
				{
					path: "/envs/prod/stacks",
					add: globals(
						expr("stacks_field", `"${terramate.name}-${global.env}"`),
					),
				},
				{
					path: "/envs/prod/stacks/stack",
					add: globals(
						expr("stack_inter", `"${global.root_field}-${global.env}-${global.stacks_field}"`),
						expr("stack_bool", "global.root_bool"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/envs/prod/stacks/stack": globals(
					str("root_field", "root-data"),
					number("root_number", 666),
					boolean("root_bool", true),
					str("root_stack_ref", "root-data-prod-stack-prod"),
					str("env_metadata", "/envs/prod/stacks/stack"),
					str("env_root_ref", "root-data"),
					str("env", "prod"),
					str("stacks_field", "stack-prod"),
					str("stack_inter", "root-data-prod-stack-prod"),
					boolean("stack_bool", true),
				),
			},
		},
		{
			name: "stack with globals referencing globals hierarchically and overriding",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{
					path: "/",
					add: globals(
						expr("stack_ref", "global.stack"),
					),
				},
				{
					path: "/stacks",
					add: globals(
						expr("stack_ref", "global.stack_other"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: globals(
						str("stack", "stack-1"),
						str("stack_other", "other stack-1"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: globals(
						str("stack", "stack-2"),
						str("stack_other", "other stack-2"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stacks/stack-1": globals(
					str("stack", "stack-1"),
					str("stack_other", "other stack-1"),
					str("stack_ref", "other stack-1"),
				),
				"/stacks/stack-2": globals(
					str("stack", "stack-2"),
					str("stack_other", "other stack-2"),
					str("stack_ref", "other stack-2"),
				),
			},
		},
		{
			name:   "unknown global reference is ignored if it is overridden",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add:  globals(expr("field", "global.wont_exist")),
				},
				{
					path: "/stack",
					add:  globals(str("field", "data")),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(str("field", "data")),
			},
		},
		{
			name:   "global reference with functions",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add:  globals(str("field", "@lala@hello")),
				},
				{
					path: "/stack",
					add: globals(
						expr("newfield", `replace(global.field, "@", "/")`),
						expr("splitfun", `split("@", global.field)[1]`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					str("field", "@lala@hello"),
					str("newfield", "/lala/hello"),
					str("splitfun", "lala"),
				),
			},
		},
		{
			name:   "global reference with successful try on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						attr("team", `{ members = ["aaa"] }`),
						expr("members", "global.team.members"),
						expr("members_try", `try(global.team.members, [])`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					attr("team", `{ members = ["aaa"] }`),
					attr("members", `["aaa"]`),
					attr("members_try", `["aaa"]`),
				),
			},
		},
		{
			name:   "global reference with failed try on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						attr("team", `{ members = ["aaa"] }`),
						expr("members_try", `try(global.team.mistake, [])`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					attr("team", `{ members = ["aaa"] }`),
					attr("members_try", "[]"),
				),
			},
		},
		{
			name:   "global reference with try on root config and value defined on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add: globals(
						expr("team_def", "global.team.def"),
						expr("team_def_try", `try(global.team.def, {})`),
					),
				},
				{
					path: "/stack",
					add: globals(
						attr("team", `{ def = { name = "awesome" } }`),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": globals(
					attr("team", `{ def = { name = "awesome" } }`),
					attr("team_def", `{ name = "awesome" }`),
					attr("team_def_try", `{ name = "awesome" }`),
				),
			},
		},
		{
			name:   "global undefined reference on root",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add:  globals(expr("field", "global.unknown")),
				},
				{
					path: "/stack",
					add:  globals(str("stack", "whatever")),
				},
			},
			wantErr: true,
		},
		{
			name:   "global undefined reference on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add:  globals(expr("field", "global.unknown")),
				},
			},
			wantErr: true,
		},
		{
			name:   "global undefined references mixed on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						expr("field_a", "global.unknown"),
						expr("field_b", "global.unknown_again"),
						str("valid", "valid"),
						expr("field_c", "global.oopsie"),
					),
				},
			},
			wantErr: true,
		},
		{
			name:   "global cyclic reference on stack",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						expr("a", "global.b"),
						expr("b", "global.c"),
						expr("c", "global.a"),
					),
				},
			},
			wantErr: true,
		},
		{
			name:   "global cyclic references across hierarchy",
			layout: []string{"s:stacks/stack"},
			globals: []globalsBlock{
				{
					path: "/",
					add:  globals(expr("a", "global.b")),
				},
				{
					path: "/stacks",
					add:  globals(expr("b", "global.c")),
				},
				{
					path: "/stacks/stack",
					add:  globals(expr("c", "global.a")),
				},
			},
			wantErr: true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, globalBlock := range tcase.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, config.Filename, globalBlock.add.String())
			}

			wantGlobals := tcase.want

			metadata := s.LoadMetadata()
			for _, stackMetadata := range metadata.Stacks {
				got, err := terramate.LoadStackGlobals(s.RootDir(), stackMetadata)

				if tcase.wantErr {
					assert.Error(t, err)
					continue
				}

				assert.NoError(t, err)

				want, ok := wantGlobals[stackMetadata.Path]
				if !ok {
					want = hclwrite.NewBlock("globals")
				}
				delete(wantGlobals, stackMetadata.Path)

				// Could have one type for globals configs and another type
				// for wanted evaluated globals, but that would make
				// globals building more annoying (two sets of functions).
				if want.HasExpressions() {
					t.Fatal("wanted globals definition contains expressions, they should be defined only by evaluated values")
					t.Errorf("wanted globals definition:\n%s\n", want)
				}

				gotAttrs := got.Attributes()
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
				t.Fatalf("wanted stack globals: %v that was not found on stacks: %v", wantGlobals, metadata.Stacks)
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
			want: hcl.ErrHCLSyntax,
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
			want: hcl.ErrHCLSyntax,
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
			want: hcl.ErrHCLSyntax,
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
			want: hcl.ErrHCLSyntax,
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
			want: terramate.ErrGlobalRedefined,
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
			want: terramate.ErrGlobalRedefined,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			metadata := s.LoadMetadata()

			for _, c := range tcase.configs {
				path := filepath.Join(s.RootDir(), c.path)
				test.AppendFile(t, path, config.Filename, c.body)
			}

			for _, stackMetadata := range metadata.Stacks {
				_, err := terramate.LoadStackGlobals(s.RootDir(), stackMetadata)
				assert.IsError(t, err, tcase.want)
			}
		})
	}
}

func TestLoadGlobalsErrorOnRelativeDir(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})

	rel, err := filepath.Rel(test.Getwd(t), s.RootDir())
	assert.NoError(t, err)

	meta := s.LoadMetadata()
	globals, err := terramate.LoadStackGlobals(rel, meta.Stacks[0])
	assert.Error(t, err, "got %v instead of error", globals)
}

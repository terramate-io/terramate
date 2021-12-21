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

package terramate_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// TODO(katcipis):
//
// - using tf functions
// - using metadata + tf functions
// - globals referencing other globals
// - err: globals referencing non-existent globals

func TestLoadGlobals(t *testing.T) {

	type (
		globalsBlock struct {
			path string
			add  *terramate.Globals
		}
		testcase struct {
			name    string
			layout  []string
			globals []globalsBlock
			want    map[string]*terramate.Globals
		}
	)

	globals := func(builders ...func(g *terramate.Globals)) *terramate.Globals {
		g := terramate.NewGlobals()
		for _, builder := range builders {
			builder(g)
		}
		return g
	}
	expr := func(key string, expr string) func(*terramate.Globals) {
		return func(g *terramate.Globals) {
			t.Helper()
			assert.NoError(t, g.AddExpr(key, expr), "building test globals")
		}
	}
	str := func(key string, val string) func(*terramate.Globals) {
		return expr(key, fmt.Sprintf("%q", val))
	}
	number := func(key string, val int64) func(*terramate.Globals) {
		return expr(key, fmt.Sprintf("%d", val))
	}
	boolean := func(key string, val bool) func(*terramate.Globals) {
		return expr(key, fmt.Sprintf("%t", val))
	}

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
			want: map[string]*terramate.Globals{
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
			want: map[string]*terramate.Globals{
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
			want: map[string]*terramate.Globals{
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
			want: map[string]*terramate.Globals{
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
			want: map[string]*terramate.Globals{
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
			want: map[string]*terramate.Globals{
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
					add:  globals(expr("stack_path", "terramate.path")),
				},
				{
					path: "/stacks/stack-2",
					add:  globals(expr("stack_path", "terramate.path")),
				},
			},
			want: map[string]*terramate.Globals{
				"/stacks/stack-1": globals(str("stack_path", "/stacks/stack-1")),
				"/stacks/stack-2": globals(str("stack_path", "/stacks/stack-2")),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, globalBlock := range tcase.globals {
				path := filepath.Join(s.BaseDir(), globalBlock.path)
				test.AppendFile(t, path, config.Filename, globalBlock.add.String())
			}

			wantGlobals := tcase.want

			metadata := s.LoadMetadata()
			for _, stackMetadata := range metadata.Stacks {
				got, err := terramate.LoadStackGlobals(s.BaseDir(), stackMetadata)
				assert.NoError(t, err)

				want, ok := wantGlobals[stackMetadata.Path]
				if !ok {
					want = terramate.NewGlobals()
				}
				delete(wantGlobals, stackMetadata.Path)

				assert.NoError(t, want.Eval(stackMetadata))

				if !got.Equal(want) {
					t.Fatalf(
						"stack %q got:\n%v\nwant:\n%v\n",
						stackMetadata.Path,
						got,
						want,
					)
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
				path := filepath.Join(s.BaseDir(), c.path)
				test.AppendFile(t, path, config.Filename, c.body)
			}

			for _, stackMetadata := range metadata.Stacks {
				_, err := terramate.LoadStackGlobals(s.BaseDir(), stackMetadata)
				assert.IsError(t, err, tcase.want)
			}
		})
	}
}

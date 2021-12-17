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
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/zclconf/go-cty/cty"
)

// TODO(katcipis):
//
// - on parent dir
// - on root dir
// - on stack + parent + root + no overriding
// - on stack + parent + root + overriding
// - multiple globals block
// - using metadata
// - using tf functions
// - using metadata + tf functions
// - err: config is not valid HCL/terramate
// - err: config has single block + redefined names
// - err: config has multiple blocks + redefined names

func TestLoadGlobals(t *testing.T) {

	type (
		config struct {
			path   string
			config string
		}
		testcase struct {
			name    string
			layout  []string
			configs []config
			want    map[string]*terramate.StackGlobals
		}
	)

	globals := func(builders ...func(g *terramate.StackGlobals)) *terramate.StackGlobals {
		g := terramate.NewStackGlobals()
		for _, builder := range builders {
			builder(g)
		}
		return g
	}
	str := func(key string, val string) func(*terramate.StackGlobals) {
		return func(g *terramate.StackGlobals) {
			g.Add(key, cty.StringVal(val))
		}
	}
	number := func(key string, val int64) func(*terramate.StackGlobals) {
		return func(g *terramate.StackGlobals) {
			g.Add(key, cty.NumberIntVal(val))
		}
	}
	boolean := func(key string, val bool) func(*terramate.StackGlobals) {
		return func(g *terramate.StackGlobals) {
			g.Add(key, cty.BoolVal(val))
		}
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
			configs: []config{
				{
					path: "/stack",
					config: `
						terramate {
						  required_version = "~> 0.0.0"
						}
						stack{}
						globals {
						  some_string = "string"
						  some_number = 777
						  some_bool = true
						}
					`,
				},
			},
			want: map[string]*terramate.StackGlobals{
				"/stack": globals(
					str("some_string", "string"),
					number("some_number", 777),
					boolean("some_bool", true),
				),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			// TODO(katcipis): build config files

			for _, stackMetadata := range s.LoadMetadata().Stacks {
				got, err := terramate.LoadStackGlobals(s.BaseDir(), stackMetadata)
				assert.NoError(t, err)

				want, ok := tcase.want[stackMetadata.Path]
				if !ok {
					want = terramate.NewStackGlobals()
				}

				if !got.Equal(want) {
					t.Fatalf(
						"stack %q got:\n%v\nwant:\n%v\n",
						stackMetadata.Path,
						got,
						want,
					)
				}
			}
		})
	}
}

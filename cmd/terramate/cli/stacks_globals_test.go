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

package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStacksGlobals(t *testing.T) {
	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name       string
			layout     []string
			workingDir string
			globals    []globalsBlock
			want       runResult
		}
	)

	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.NewBuilder("globals", builders...)
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
			name:   "single stack with a global",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: globals(
						str("str", "string"),
						number("number", 777),
						boolean("bool", true),
					),
				},
			},
			want: runResult{
				Stdout: `
stack "/stack":
	global.str    = "string"
	global.number = 777
	global.bool   = true
`,
			},
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

			ts := newCLI(t, s.RootDir())
			assertRunResult(t, ts.run("stacks", "globals"), tcase.want)
		})
	}
}

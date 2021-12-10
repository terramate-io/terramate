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
	"testing"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCLIRunOrder(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		want   runResult
	}

	for _, tc := range []testcase{
		{
			name: "one stack",
			layout: []string{
				"s:stack-a",
			},
			want: runResult{
				Stdout: `stack-a
`,
			},
		},
		{
			name: "empty ordering",
			layout: []string{
				"s:stack:after=[]",
			},
			want: runResult{
				Stdout: `stack
`,
			},
		},
		{
			name: "independent stacks, consistent ordering (reverse-lexicographic)",
			layout: []string{
				"s:batatinha",
				"s:frita",
				"s:1",
				"s:2",
				"s:3",
				"s:boom",
			},
			want: runResult{
				Stdout: `frita
boom
batatinha
3
2
1
`,
			},
		},
		{
			name: "stack-b after stack-a",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["../stack-a"]`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
`,
			},
		},
		{
			name: "stack-c after stack-b after stack-a",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-c:after=["../stack-b"]`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
stack-c
`,
			},
		},
		{
			name: "stack-a after stack-b after stack-c",
			layout: []string{
				"s:stack-c",
				`s:stack-b:after=["../stack-c"]`,
				`s:stack-a:after=["../stack-b"]`,
			},
			want: runResult{
				Stdout: `stack-c
stack-b
stack-a
`,
			},
		},
		{
			name: "stack-a after stack-b",
			layout: []string{
				`s:stack-a:after=["../stack-b"]`,
				`s:stack-b`,
			},
			want: runResult{
				Stdout: `stack-b
stack-a
`,
			},
		},
		{
			name: "stack-a after (stack-b, stack-c, stack-d)",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: runResult{
				Stdout: `stack-d
stack-c
stack-b
stack-a
`,
			},
		},
		{
			name: "stack-c after stack-b after stack-a, stack-d after stack-z",
			layout: []string{
				`s:stack-c:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-a`,
				`s:stack-d:after=["../stack-z"]`,
				`s:stack-z`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
stack-c
stack-z
stack-d
`,
			},
		},
		{
			name: "stack-c after stack-b after stack-a, stack-d after stack-b",
			layout: []string{
				`s:stack-c:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-a`,
				`s:stack-d:after=["../stack-b"]`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
stack-d
stack-c
`,
			},
		},
		{
			name: "stack-c after stack-b after stack-a, stack-z after stack-d after stack-b",
			layout: []string{
				`s:stack-c:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-a`,
				`s:stack-z:after=["../stack-d"]`,
				`s:stack-d:after=["../stack-b"]`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
stack-d
stack-z
stack-c
`,
			},
		},
		{
			name: "stack-g after stack-c after stack-b after stack-a, stack-z after stack-d after stack-b",
			layout: []string{
				`s:stack-g:after=["../stack-c"]`,
				`s:stack-c:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-a`,
				`s:stack-z:after=["../stack-d"]`,
				`s:stack-d:after=["../stack-b"]`,
			},
			want: runResult{
				Stdout: `stack-a
stack-b
stack-d
stack-z
stack-c
stack-g
`,
			},
		},
		{
			name: "stack-a after (stack-b, stack-c), stack-b after (stack-d, stack-f), stack-c after (stack-g, stack-h)",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b:after=["../stack-d", "../stack-f"]`,
				`s:stack-c:after=["../stack-g", "../stack-h"]`,
				`s:stack-d`,
				`s:stack-f`,
				`s:stack-g`,
				`s:stack-h`,
			},
			want: runResult{
				Stdout: `stack-h
stack-g
stack-c
stack-f
stack-d
stack-b
stack-a
`,
			},
		},
		{
			name: "stack-a after stack-a - fails",
			layout: []string{
				`s:stack-a:after=["../stack-a"]`,
			},
			want: runResult{
				Error:        terramate.ErrRunCycleDetected,
				IgnoreStderr: true,
			},
		},
		{
			name: "CYCLE",
			layout: []string{
				`s:stack-a:after=["."]`,
			},
			want: runResult{
				Error:        terramate.ErrRunCycleDetected,
				IgnoreStderr: true,
			},
		},
		{
			name: "stack-a after stack-b after stack-c after stack-a - fails",
			layout: []string{
				`s:stack-a:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-c"]`,
				`s:stack-c:after=["../stack-a"]`,
			},
			want: runResult{
				Error:        terramate.ErrRunCycleDetected,
				IgnoreStderr: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.BaseDir())
			assertRunResult(t, cli.run("run-order"), tc.want)
		})
	}
}

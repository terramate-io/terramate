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
	"os"
	"testing"

	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCLIList(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		want   runResult
	}

	for _, tc := range []testcase{
		{
			name: "no stack",
		},
		{
			name: "no stack, lots of dirs",
			layout: []string{
				"d:dir1/a/b/c",
				"d:dir2/a/b/c/x/y",
				"d:last/dir",
			},
		},
		{
			name:   "single stack",
			layout: []string{"s:stack"},
			want: runResult{
				Stdout: "/stack\n",
			},
		},
		{
			name: "single stack down deep inside directories",
			layout: []string{
				"d:lots",
				"d:of",
				"d:directories",
				"d:lots/lots",
				"d:of/directories/without/any/stack",
				"d:but",
				"s:there/is/a/very/deep/hidden/stack/here",
				"d:more",
				"d:waste/directories",
			},
			want: runResult{
				Stdout: "/there/is/a/very/deep/hidden/stack/here\n",
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: runResult{
				Stdout: "/1\n/2\n/3\n",
			},
		},
		{
			name: "multiple stacks at multiple levels",
			layout: []string{
				"s:1",
				"s:2",
				"s:z/a",
				"s:x/b",
				"d:not-stack",
				"d:something/else/uninportant",
				"s:3/x/y/z",
			},
			want: runResult{
				Stdout: `/1
/2
/3/x/y/z
/x/b
/z/a
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.BaseDir())
			assertRunResult(t, cli.run("list"), tc.want)
		})
	}
}

func TestListNoSuchFile(t *testing.T) {
	notExists := test.NonExistingDir(t)
	cli := newCLI(t, notExists)

	// errors from the manager are not logged in stderr
	assertRunResult(t, cli.run("list"), runResult{
		Error: os.ErrNotExist,
	})
}

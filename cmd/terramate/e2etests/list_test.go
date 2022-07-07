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

package e2etest

import (
	"testing"

	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

type testcase struct {
	name   string
	layout []string
	want   runExpected
}

func listTestcases() []testcase {
	return []testcase{
		{
			name: "no stack",
		},
		{
			name: "dot directories ignored",
			layout: []string{
				"f:.stack/stack.tm:stack {}",
			},
		},
		{
			name: "dot files ignored",
			layout: []string{
				"f:stack/.stack.tm:stack {}",
			},
		},
		{
			name: "dot directories ignored in isLeaf check",
			layout: []string{
				"s:stack",
				"f:stack/.substack/stack.tm:stack {}",
			},
			want: runExpected{
				Stdout: "stack\n",
			},
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
			want: runExpected{
				Stdout: "stack\n",
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
			want: runExpected{
				Stdout: "there/is/a/very/deep/hidden/stack/here\n",
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: runExpected{
				Stdout: "1\n2\n3\n",
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
			want: runExpected{
				Stdout: `1
2
3/x/y/z
x/b
z/a
`,
			},
		},
	}
}

func TestListStackWithDefinitionOnNonDefaultFilename(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"d:stack"})
	stackDir := s.DirEntry("stack")
	stackDir.CreateFile("stack.tm", "stack {}")

	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListStackWithNoTerramateBlock(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})
	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListNoSuchFile(t *testing.T) {
	notExists := test.NonExistingDir(t)
	cli := newCLI(t, notExists)

	// errors from the manager are not logged in stderr
	assertRunResult(t, cli.listStacks(), runExpected{
		Status:      1,
		StderrRegex: "no such file or directory",
	})
}

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

package e2etest

import (
	"testing"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCliMetadata(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		wd     string
		want   runExpected
	}

	for _, tc := range []testcase{
		{
			name: "no stacks",
		},
		{
			name: "one stack, wd = root",
			layout: []string{
				"s:stack",
			},
			want: runExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.path.absolute="/stack"
	terramate.stack.path.basename="stack"
	terramate.stack.path.relative="stack"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "one stack with ID",
			layout: []string{
				"s:stack:id=unique-id",
			},
			want: runExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.id="unique-id"
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.path.absolute="/stack"
	terramate.stack.path.basename="stack"
	terramate.stack.path.relative="stack"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "multiple stacks, wd = root",
			layout: []string{
				"s:stack1",
				"s:stack2",
				"s:somedir/stack3",
				"s:somedir/stack4",
			},
			want: runExpected{
				Stdout: `Available metadata:

stack "/somedir/stack3":
	terramate.stack.name="stack3"
	terramate.stack.description=""
	terramate.stack.path.absolute="/somedir/stack3"
	terramate.stack.path.basename="stack3"
	terramate.stack.path.relative="somedir/stack3"
	terramate.stack.path.to_root="../.."

stack "/somedir/stack4":
	terramate.stack.name="stack4"
	terramate.stack.description=""
	terramate.stack.path.absolute="/somedir/stack4"
	terramate.stack.path.basename="stack4"
	terramate.stack.path.relative="somedir/stack4"
	terramate.stack.path.to_root="../.."

stack "/stack1":
	terramate.stack.name="stack1"
	terramate.stack.description=""
	terramate.stack.path.absolute="/stack1"
	terramate.stack.path.basename="stack1"
	terramate.stack.path.relative="stack1"
	terramate.stack.path.to_root=".."

stack "/stack2":
	terramate.stack.name="stack2"
	terramate.stack.description=""
	terramate.stack.path.absolute="/stack2"
	terramate.stack.path.basename="stack2"
	terramate.stack.path.relative="stack2"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "multiple stacks, wd = /stack1",
			layout: []string{
				"s:stack1",
				"s:stack2",
				"s:somedir/stack3",
				"s:somedir/stack4",
			},
			wd: "/stack1",
			want: runExpected{
				Stdout: `Available metadata:

stack "/stack1":
	terramate.stack.name="stack1"
	terramate.stack.description=""
	terramate.stack.path.absolute="/stack1"
	terramate.stack.path.basename="stack1"
	terramate.stack.path.relative="stack1"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "multiple stacks, wd = /somedir",
			layout: []string{
				"s:stack1",
				"s:stack2",
				"s:somedir/stack3",
				"s:somedir/stack4",
			},
			wd: "/somedir",
			want: runExpected{
				Stdout: `Available metadata:

stack "/somedir/stack3":
	terramate.stack.name="stack3"
	terramate.stack.description=""
	terramate.stack.path.absolute="/somedir/stack3"
	terramate.stack.path.basename="stack3"
	terramate.stack.path.relative="somedir/stack3"
	terramate.stack.path.to_root="../.."

stack "/somedir/stack4":
	terramate.stack.name="stack4"
	terramate.stack.description=""
	terramate.stack.path.absolute="/somedir/stack4"
	terramate.stack.path.basename="stack4"
	terramate.stack.path.relative="somedir/stack4"
	terramate.stack.path.to_root="../.."
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, project.AbsPath(s.RootDir(), tc.wd))
			assertRunResult(t, cli.run("experimental", "metadata"), tc.want)
		})
	}
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCliMetadata(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		wd     string
		flags  []string
		want   RunExpected
	}

	for _, tcase := range []testcase{
		{
			name: "no stacks",
		},
		{
			name: "stack at /, wd = root",
			layout: []string{
				"s:/",
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/":
	terramate.stack.name="sandbox"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/"
	terramate.stack.path.basename="/"
	terramate.stack.path.relative=""
	terramate.stack.path.to_root="."
`,
			},
		},
		{
			name: "one stack, wd = root",
			layout: []string{
				"s:stack",
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.tags=[]
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
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.id="unique-id"
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.tags=[]
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
			want: RunExpected{
				Stdout: `Available metadata:

stack "/somedir/stack3":
	terramate.stack.name="stack3"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/somedir/stack3"
	terramate.stack.path.basename="stack3"
	terramate.stack.path.relative="somedir/stack3"
	terramate.stack.path.to_root="../.."

stack "/somedir/stack4":
	terramate.stack.name="stack4"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/somedir/stack4"
	terramate.stack.path.basename="stack4"
	terramate.stack.path.relative="somedir/stack4"
	terramate.stack.path.to_root="../.."

stack "/stack1":
	terramate.stack.name="stack1"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/stack1"
	terramate.stack.path.basename="stack1"
	terramate.stack.path.relative="stack1"
	terramate.stack.path.to_root=".."

stack "/stack2":
	terramate.stack.name="stack2"
	terramate.stack.description=""
	terramate.stack.tags=[]
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
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack1":
	terramate.stack.name="stack1"
	terramate.stack.description=""
	terramate.stack.tags=[]
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
			want: RunExpected{
				Stdout: `Available metadata:

stack "/somedir/stack3":
	terramate.stack.name="stack3"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/somedir/stack3"
	terramate.stack.path.basename="stack3"
	terramate.stack.path.relative="somedir/stack3"
	terramate.stack.path.to_root="../.."

stack "/somedir/stack4":
	terramate.stack.name="stack4"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/somedir/stack4"
	terramate.stack.path.basename="stack4"
	terramate.stack.path.relative="somedir/stack4"
	terramate.stack.path.to_root="../.."
`,
			},
		},
		{
			name: "one stack with empty tags",
			layout: []string{
				`s:stack:tags=[]`,
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/stack"
	terramate.stack.path.basename="stack"
	terramate.stack.path.relative="stack"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "one stack with tags",
			layout: []string{
				`s:stack:tags=["tag1", "tag2"]`,
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.tags=["tag1","tag2"]
	terramate.stack.path.absolute="/stack"
	terramate.stack.path.basename="stack"
	terramate.stack.path.relative="stack"
	terramate.stack.path.to_root=".."
`,
			},
		},
		{
			name: "one stack with parent stack",
			layout: []string{
				`s:parent:id=parent-stack`,
				`s:parent/child`,
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/parent":
	terramate.stack.id="parent-stack"
	terramate.stack.name="parent"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/parent"
	terramate.stack.path.basename="parent"
	terramate.stack.path.relative="parent"
	terramate.stack.path.to_root=".."

stack "/parent/child":
	terramate.stack.name="child"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/parent/child"
	terramate.stack.path.basename="child"
	terramate.stack.path.relative="parent/child"
	terramate.stack.path.to_root="../.."
`,
			},
		},
		{
			name: "one stack with parent stack separated by directories",
			layout: []string{
				`s:parent:id=parent-stack`,
				`s:parent/some/child/stack`,
			},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/parent":
	terramate.stack.id="parent-stack"
	terramate.stack.name="parent"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/parent"
	terramate.stack.path.basename="parent"
	terramate.stack.path.relative="parent"
	terramate.stack.path.to_root=".."

stack "/parent/some/child/stack":
	terramate.stack.name="stack"
	terramate.stack.description=""
	terramate.stack.tags=[]
	terramate.stack.path.absolute="/parent/some/child/stack"
	terramate.stack.path.basename="stack"
	terramate.stack.path.relative="parent/some/child/stack"
	terramate.stack.path.to_root="../../../.."
`,
			},
		},
		{
			name: "tags",
			layout: []string{
				`s:stack1:tags=["tag1","tag2"]`,
				`s:stack1/sub1:tags=["tag2"]`,
				`s:stack2:tags=["tag2"]`,
			},
			wd:    "/stack1",
			flags: []string{"--tags=tag2", "--no-tags=tag1"},
			want: RunExpected{
				Stdout: `Available metadata:

stack "/stack1/sub1":
	terramate.stack.name="sub1"
	terramate.stack.description=""
	terramate.stack.tags=["tag2"]
	terramate.stack.path.absolute="/stack1/sub1"
	terramate.stack.path.basename="sub1"
	terramate.stack.path.relative="stack1/sub1"
	terramate.stack.path.to_root="../.."
`,
			},
		},
	} {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)

			args := []string{"debug", "show", "metadata"}
			if len(tc.flags) > 0 {
				args = append(args, tc.flags...)
			}

			cli := NewCLI(t, project.AbsPath(s.RootDir(), tc.wd))
			AssertRunResult(t, cli.Run(args...), tc.want)
		})
	}
}

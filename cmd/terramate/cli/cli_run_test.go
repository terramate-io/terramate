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
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate"

	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/run/dag"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCLIRunOrder(t *testing.T) {
	type testcase struct {
		name    string
		layout  []string
		changed bool
		want    runExpected
	}

	for _, tc := range []testcase{
		{
			name: "one stack",
			layout: []string{
				"s:stack-a",
			},
			want: runExpected{
				Stdout: `stack-a
`,
			},
		},
		{
			name: "empty ordering",
			layout: []string{
				"s:stack:after=[]",
			},
			want: runExpected{
				Stdout: `stack
`,
			},
		},
		{
			name: "independent stacks, consistent ordering (lexicographic)",
			layout: []string{
				"s:batatinha",
				"s:frita",
				"s:1",
				"s:2",
				"s:3",
				"s:boom",
			},
			want: runExpected{
				Stdout: `1
2
3
batatinha
boom
frita
`,
			},
		},
		{
			name: "stack-b after stack-a",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["../stack-a"]`,
			},
			want: runExpected{
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
			want: runExpected{
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
			want: runExpected{
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
			want: runExpected{
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
			want: runExpected{
				Stdout: `stack-b
stack-c
stack-d
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
			want: runExpected{
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
			want: runExpected{
				Stdout: `stack-a
stack-b
stack-c
stack-d
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
			want: runExpected{
				Stdout: `stack-a
stack-b
stack-c
stack-d
stack-z
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
			want: runExpected{
				Stdout: `stack-a
stack-b
stack-c
stack-d
stack-g
stack-z
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
			want: runExpected{
				Stdout: `stack-d
stack-f
stack-b
stack-g
stack-h
stack-c
stack-a
`,
			},
		},
		{
			name: "stack-z after (stack-a, stack-b, stack-c, stack-d), stack-a after (stack-b, stack-c)",
			layout: []string{
				`s:stack-z:after=["../stack-a", "../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: runExpected{
				Stdout: `stack-b
stack-c
stack-a
stack-d
stack-z
`,
			},
		},
		{
			name: "stack-z after (stack-a, stack-b, stack-c, stack-d), stack-a after (stack-x, stack-y)",
			layout: []string{
				`s:stack-z:after=["../stack-a", "../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-x", "../stack-y"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
				`s:stack-x`,
				`s:stack-y`,
			},
			want: runExpected{
				Stdout: `stack-x
stack-y
stack-a
stack-b
stack-c
stack-d
stack-z
`,
			},
		},
		{
			name: "stack-a after stack-a - fails",
			layout: []string{
				`s:stack-a:after=["../stack-a"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: dag.ErrCycleDetected.Error(),
			},
		},
		{
			name: "stack-a after . - fails",
			layout: []string{
				`s:stack-a:after=["."]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: dag.ErrCycleDetected.Error(),
			},
		},
		{
			name: "stack-a after stack-b after stack-c after stack-a - fails",
			layout: []string{
				`s:stack-a:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-c"]`,
				`s:stack-c:after=["../stack-a"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: dag.ErrCycleDetected.Error(),
			},
		},
		{
			name: "1 after 4 after 20 after 1 - fails",
			layout: []string{
				`s:1:after=["../2", "../3", "../4", "../5", "../6", "../7"]`,
				`s:2:after=["../12", "../13", "../14", "../15", "../16"]`,
				`s:3:after=["../2", "../4"]`,
				`s:4:after=["../6", "../20"]`,
				`s:5`,
				`s:6`,
				`s:7`,
				`s:8`,
				`s:9`,
				`s:10`,
				`s:11`,
				`s:12`,
				`s:13`,
				`s:14`,
				`s:15`,
				`s:16`,
				`s:17`,
				`s:18`,
				`s:19`,
				`s:20:after=["../10", "../1"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: dag.ErrCycleDetected.Error(),
			},
		},
		{
			name: `stack-z after (stack-b, stack-c, stack-d)
				   stack-a after stack-c
				   stack-b before stack-a`,
			layout: []string{
				`s:stack-z:after=["../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-c"]`,
				`s:stack-b:before=["../stack-a"]`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: runExpected{
				Stdout: `stack-b
stack-c
stack-a
stack-d
stack-z
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.RootDir())
			args := []string{"plan", "run-order"}
			if tc.changed {
				args = append(args, "--changed")
			}
			assertRunResult(t, cli.run(args...), tc.want)
		})
	}
}

func TestRunOrderNotChangedStackIgnored(t *testing.T) {
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	// stack must run after stack2 but stack2 didn't change.

	stack2 := s.CreateStack("stack2")

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")
	stackConfig := hcl.NewConfig(terramate.DefaultVersionConstraint())
	stackConfig.Stack = &hcl.Stack{
		After: []string{stack2.Path()},
	}
	stack.WriteConfig(stackConfig)

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	cli := newCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	assertRunResult(t, cli.run("stacks", "list", "--changed"), runExpected{Stdout: wantList})

	cat := test.LookPath(t, "cat")
	wantRun := mainTfContents

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: wantRun})

	wantRun = mainTfContents

	cli = newCLI(t, stack.Path())
	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: wantRun})

	cli = newCLI(t, stack2.Path())
	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{})
}

func TestRunOrderAllChangedStacksExecuted(t *testing.T) {
	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	// stack2 must run after stack and both changed.

	s := sandbox.New(t)

	stack2 := s.CreateStack("stack2")
	stack2MainTf := stack2.CreateFile(mainTfFileName, "# some code")

	stack := s.CreateStack("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")
	stackConfig := hcl.NewConfig(terramate.DefaultVersionConstraint())
	stackConfig.Stack = &hcl.Stack{
		After: []string{stack2.Path()},
	}
	stack.WriteConfig(stackConfig)

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	stack2MainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	cli := newCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n" + stack2.RelPath() + "\n"
	assertRunResult(t, cli.run("stacks", "list", "--changed"), runExpected{Stdout: wantList})

	cat := test.LookPath(t, "cat")
	wantRun := fmt.Sprintf(
		"%s%s",
		mainTfContents,
		mainTfContents,
	)

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: wantRun})
}

func TestRunLogsUserCommand(t *testing.T) {
	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testfile := stack.CreateFile("test", "")

	cli := newCLIWithLogLevel(t, s.RootDir(), "info")
	assertRunResult(t, cli.run("run", "cat", testfile.Path()), runExpected{
		StderrRegex: `cmd="cat /`,
	})
}

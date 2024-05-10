// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestOrderGraphAfter(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		want   RunExpected
	}

	for _, tcase := range []testcase{
		{
			name: "one stack, no order",
			layout: []string{
				`s:stack`,
			},
			want: RunExpected{
				Stdout:        `digraph  {n1[label="stack"];}`,
				FlattenStdout: true,
			},
		},
		{
			name: "two stacks, no order",
			layout: []string{
				`s:stack1`,
				`s:stack2`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack1"];
					n2[label="stack2"];
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "stack after anotherstack",
			layout: []string{
				`s:stack:after=["../anotherstack"]`,
				`s:anotherstack`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="anotherstack"];
					n2[label="stack"];
					n1->n2;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "stack after anotherstack (root path)",
			layout: []string{
				`s:stack:after=["/anotherstack"]`,
				`s:anotherstack`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="anotherstack"];
					n2[label="stack"];
					n1->n2;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "stack-a after (stack-b, stack-c)",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b`,
				`s:stack-c`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n2->n1;
					n3->n1;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "stack-a after (stack-b, stack-c); stack-b after stack-c",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b:after=["../stack-c"]`,
				`s:stack-c`,
			},
			want: RunExpected{
				Stdout: `digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n2->n1;
					n3->n2;
					n3->n1;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "complex",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c", "../stack-d", "../stack-e"]`,
				`s:stack-b:after=["../stack-f", "../stack-c"]`,
				`s:stack-c:after=["../stack-f", "../stack-g"]`,
				`s:stack-d`,
				`s:stack-e:after=["../stack-x"]`,
				`s:stack-f`,
				`s:stack-g`,
				`s:stack-x`,
			},
			want: RunExpected{
				Stdout: `digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n6[label="stack-d"];
					n7[label="stack-e"];
					n4[label="stack-f"];
					n5[label="stack-g"];
					n8[label="stack-x"];
					n2->n1;
					n3->n2;
					n3->n1;
					n6->n1;
					n7->n1;
					n4->n3;
					n4->n2;
					n5->n3;
					n8->n7;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "cycle: stack-a after stack-a",
			layout: []string{
				`s:stack-a:after=["../stack-a"]`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n1->n1[color="red"];
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "cycle: stack-a after stack-b after stack-a",
			layout: []string{
				`s:stack-a:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {n1[label="stack-a"];
					n2[label="stack-b"];
					n1->n2[color="red"];
					n2->n1;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "multiple cycles: stack-a after (stack-b, stack-c); stack-b after stack-a; stack-c after stack-a",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-c:after=["../stack-a"]`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n1->n2[color="red"];
					n1->n3[color="red"];
					n2->n1;
					n3->n1;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "nodes can appear multiple times in different branches",
			layout: []string{
				`s:stack-z:after=["../stack-a", "../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n4[label="stack-d"];
					n5[label="stack-z"];
					n1->n5;
					n2->n1;
					n2->n5;
					n3->n1;
					n3->n5;
					n4->n5;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "multi-branch at several levels",
			layout: []string{
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b:after=["../stack-d", "../stack-f"]`,
				`s:stack-c:after=["../stack-g", "../stack-h"]`,
				`s:stack-d`,
				`s:stack-f`,
				`s:stack-g`,
				`s:stack-h`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n5[label="stack-c"];
					n3[label="stack-d"];
					n4[label="stack-f"];
					n6[label="stack-g"];
					n7[label="stack-h"];
					n2->n1;
					n5->n1;
					n3->n2;
					n4->n2;
					n6->n5;
					n7->n5;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "multi-branch at several levels - way complex",
			layout: []string{
				`s:stack-z:after=["../stack-a", "../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-b", "../stack-c"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n4[label="stack-d"];
					n5[label="stack-z"];
					n1->n5;
					n2->n1;
					n2->n5;
					n3->n1;
					n3->n5;
					n4->n5;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "multi-branch - independent ones",
			layout: []string{
				`s:stack-z:after=["../stack-a", "../stack-b", "../stack-c", "../stack-d"]`,
				`s:stack-a:after=["../stack-x", "../stack-y"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
				`s:stack-x`,
				`s:stack-y`,
			},
			want: RunExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n4[label="stack-b"];
					n5[label="stack-c"];
					n6[label="stack-d"];
					n2[label="stack-x"];
					n3[label="stack-y"];
					n7[label="stack-z"];
					n1->n7;
					n4->n7;
					n5->n7;
					n6->n7;
					n2->n1;
					n3->n1;
				}`,
				FlattenStdout: true,
			},
		},
	} {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)
			cli := NewCLI(t, s.RootDir())
			AssertRunResult(t, cli.StacksRunGraph(), tc.want)
		})
	}
}

func TestExperimentalRunOrderNotChangedStackIgnored(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	// stack must run after stack2 but stack2 didn't change.
	s.BuildTree([]string{
		`s:stack:after=["/stack2"]`,
		"s:stack2",
	})

	stack := s.DirEntry("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	cli := NewCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})
	AssertRunResult(t, cli.Run("--changed", "experimental", "run-order"),
		RunExpected{
			Stdout: "/" + wantList,
		})
}

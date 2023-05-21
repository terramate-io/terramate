// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"strings"
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestOrderGraphAfter(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		want   runExpected
	}

	for _, tcase := range []testcase{
		{
			name: "one stack, no order",
			layout: []string{
				`s:stack`,
			},
			want: runExpected{
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
			want: runExpected{
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="anotherstack"];
					n2[label="stack"];
					n2->n1;
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="anotherstack"];
					n2[label="stack"];
					n2->n1;
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n1->n2;
					n1->n3;
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
			want: runExpected{
				Stdout: `digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n1->n2;
					n1->n3;
					n2->n3;
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
			want: runExpected{
				Stdout: `digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n6[label="stack-d"];
					n7[label="stack-e"];
					n4[label="stack-f"];
					n5[label="stack-g"];
					n8[label="stack-x"];
					n1->n2;
					n1->n3;
					n1->n6;
					n1->n7;
					n2->n3;
					n2->n4;
					n3->n4;
					n3->n5;
					n7->n8;
				}`,
				FlattenStdout: true,
			},
		},
		{
			name: "cycle: stack-a after stack-a",
			layout: []string{
				`s:stack-a:after=["../stack-a"]`,
			},
			want: runExpected{
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
			want: runExpected{
				Stdout: `
				digraph  {n1[label="stack-a"];
					n2[label="stack-b"];
					n1->n2;
					n2->n1[color="red"];
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n1->n2;
					n1->n3;
					n2->n1[color="red"];
					n3->n1[color="red"];
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n4[label="stack-d"];
					n5[label="stack-z"];
					n1->n2;
					n1->n3;
					n5->n1;
					n5->n2;
					n5->n3;
					n5->n4;
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n5[label="stack-c"];
					n3[label="stack-d"];
					n4[label="stack-f"];
					n6[label="stack-g"];
					n7[label="stack-h"];
					n1->n2;
					n1->n5;
					n2->n3;
					n2->n4;
					n5->n6;
					n5->n7;
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n2[label="stack-b"];
					n3[label="stack-c"];
					n4[label="stack-d"];
					n5[label="stack-z"];
					n1->n2;
					n1->n3;
					n5->n1;
					n5->n2;
					n5->n3;
					n5->n4;
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
			want: runExpected{
				Stdout: `
				digraph  {
					n1[label="stack-a"];
					n4[label="stack-b"];
					n5[label="stack-c"];
					n6[label="stack-d"];
					n2[label="stack-x"];
					n3[label="stack-y"];
					n7[label="stack-z"];
					n1->n2;
					n1->n3;
					n7->n1;
					n7->n4;
					n7->n5;
					n7->n6;
				}`,
				FlattenStdout: true,
			},
		},
	} {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			cli := newCLI(t, s.RootDir())
			assertRunResult(t, cli.stacksRunGraph(), tc.want)
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

	cli := newCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: wantList})
	assertRunResult(t, cli.run("--changed", "experimental", "run-order"),
		runExpected{
			Stdout: "/" + wantList,
		})
}

// remove tabs and newlines
func flatten(s string) string {
	return strings.Replace((strings.Replace(s, "\n", "", -1)), "\t", "", -1)
}

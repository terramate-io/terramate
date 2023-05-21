// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

type selectionTestcase struct {
	name         string
	layout       []string
	wd           string
	filterTags   []string
	filterNoTags []string
	want         runExpected
}

func TestCLIRunOrder(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name         string
		layout       []string
		filterTags   []string
		filterNoTags []string
		workingDir   string
		want         runExpected
	}

	for _, tc := range []testcase{
		{
			name: "one stack",
			layout: []string{
				"s:stack-a",
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
				),
			},
		},
		{
			name: "empty ordering",
			layout: []string{
				"s:stack:after=[]",
			},
			want: runExpected{
				Stdout: listStacks(
					`/stack`,
				),
			},
		},
		{
			name: "after non-existent path",
			layout: []string{
				fmt.Sprintf("s:stack:after=[%q]", test.NonExistingDir(t)),
			},
			want: runExpected{
				Stdout: listStacks(
					`/stack`,
				),
			},
		},
		{
			name: "after regular file",
			layout: []string{
				fmt.Sprintf("s:stack:after=[%q]", test.WriteFile(t, "", "test.txt", `bleh`)),
			},
			want: runExpected{
				Stdout: listStacks(
					`/stack`,
				),
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
				Stdout: listStacks(
					"/1",
					"/2",
					"/3",
					"/batatinha",
					"/boom",
					"/frita",
				),
			},
		},
		{
			name: "independent stacks inside other stacks gives consistent ordering (lexicographic by path)",
			layout: []string{
				"s:stacks",
				"s:stacks/A",
				"s:stacks/B",
				"s:stacks/A/AA",
				"s:stacks/B/BA",
				"s:stacks/A/AA/AAA",
			},
			want: runExpected{
				Stdout: listStacks(
					"/stacks",
					"/stacks/A",
					"/stacks/A/AA",
					"/stacks/A/AA/AAA",
					"/stacks/B",
					"/stacks/B/BA",
				),
			},
		},
		{
			name: "stack-b after stack-a (relpaths)",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["../stack-a"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-b after stack-a (abspaths)",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["/stack-a"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-c after stack-b after stack-a (relpaths)",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-c:after=["../stack-b"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
				),
			},
		},
		{
			name: "stack-c after stack-b after stack-a (abspaths)",
			layout: []string{
				"s:stack-a",
				`s:stack-b:after=["/stack-a"]`,
				`s:stack-c:after=["/stack-b"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
				),
			},
		},
		{
			name: "stack-a after stack-b after stack-c (relpaths)",
			layout: []string{
				"s:stack-c",
				`s:stack-b:after=["../stack-c"]`,
				`s:stack-a:after=["../stack-b"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-c",
					"/stack-b",
					"/stack-a",
				),
			},
		},
		{
			name: "stack-a after stack-b after stack-c (abspaths)",
			layout: []string{
				"s:stack-c",
				`s:stack-b:after=["/stack-c"]`,
				`s:stack-a:after=["/stack-b"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-c",
					"/stack-b",
					"/stack-a",
				),
			},
		},
		{
			name: "stack-a after stack-b (relpaths)",
			layout: []string{
				`s:stack-a:after=["../stack-b"]`,
				`s:stack-b`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
					"/stack-a",
				),
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
				Stdout: listStacks(
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-a",
				),
			},
		},
		{
			name: "stack-a after (stack-b, stack-c, stack-d) (abspaths)",
			layout: []string{
				`s:stack-a:after=["/stack-b", "/stack-c", "/stack-d"]`,
				`s:stack-b`,
				`s:stack-c`,
				`s:stack-d`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-a",
				),
			},
		},
		{
			name: "stack-c after stack-b after stack-a, stack-d after stack-z (relpaths)",
			layout: []string{
				`s:stack-c:after=["../stack-b"]`,
				`s:stack-b:after=["../stack-a"]`,
				`s:stack-a`,
				`s:stack-d:after=["../stack-z"]`,
				`s:stack-z`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-z",
					"/stack-d",
				),
			},
		},
		{
			name: "stack-c after stack-b after stack-a, stack-d after stack-z (abspaths)",
			layout: []string{
				`s:stack-c:after=["/stack-b"]`,
				`s:stack-b:after=["/stack-a"]`,
				`s:stack-a`,
				`s:stack-d:after=["/stack-z"]`,
				`s:stack-z`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-z",
					"/stack-d",
				),
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
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
				),
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
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-z",
				),
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
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-g",
					"/stack-z",
				),
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
				Stdout: listStacks(
					"/stack-d",
					"/stack-f",
					"/stack-b",
					"/stack-g",
					"/stack-h",
					"/stack-c",
					"/stack-a",
				),
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
				Stdout: listStacks(
					"/stack-b",
					"/stack-c",
					"/stack-a",
					"/stack-d",
					"/stack-z",
				),
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
				Stdout: listStacks(
					"/stack-x",
					"/stack-y",
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-z",
				),
			},
		},
		{
			name: "stack-a after stack-a - fails",
			layout: []string{
				`s:stack-a:after=["../stack-a"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
			},
		},
		{
			name: "stack-a after . - fails",
			layout: []string{
				`s:stack-a:after=["."]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
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
				StderrRegex: string(dag.ErrCycleDetected),
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
				StderrRegex: string(dag.ErrCycleDetected),
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
				Stdout: listStacks(
					"/stack-b",
					"/stack-c",
					"/stack-a",
					"/stack-d",
					"/stack-z",
				),
			},
		},
		{
			name: `run order selects only stacks inside working dir`,
			layout: []string{
				`s:stacks/stack-a:after=["/stacks/stack-b", "/parent-stack"]`,
				`s:stacks/stack-b:before=["/parent-stack"]`,
				`s:parent-stack`,
			},
			workingDir: "stacks",
			want: runExpected{
				Stdout: listStacks(
					"/stacks/stack-b",
					"/stacks/stack-a",
				),
			},
		},
		{
			name: "stack-b after stack-a after parent (implicit)",
			layout: []string{
				`s:parent/stack-b:after=["/parent/stack-a"]`,
				`s:parent/stack-a`,
				`s:parent`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/parent",
					"/parent/stack-a",
					"/parent/stack-b",
				),
			},
		},
		{
			name: "grand parent before parent before child (implicit)",
			layout: []string{
				`s:grand-parent/parent/child`,
				`s:grand-parent/parent`,
				`s:grand-parent`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/grand-parent",
					"/grand-parent/parent",
					"/grand-parent/parent/child",
				),
			},
		},
		{
			name: "child stack CANNOT have explicit after clause to parent",
			layout: []string{
				`s:stacks`,
				`s:stacks/child:after=["/stacks"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
			},
		},
		{
			name: "child stack can never run before the parent - cycle",
			layout: []string{
				`s:stacks`,
				`s:stacks/child:before=["/stacks"]`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
			},
		},
		{
			name: "parent stack can never run after the child - cycle",
			layout: []string{
				`s:stacks:after=["/stacks/child"]`,
				`s:stacks/child`,
			},
			want: runExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
			},
		},
		{
			name: "after directory containing stacks",
			layout: []string{
				`d:dir`,
				`s:dir/s1`,
				`s:dir/s2`,
				`s:dir/s3`,
				`s:stack:after=["/dir"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/dir/s1",
					"/dir/s2",
					"/dir/s3",
					"/stack",
				),
			},
		},
		{
			name: "after stack containing sub-stacks",
			layout: []string{
				`s:stack:after=["/parent"]`,
				`s:parent/s3`,
				`s:parent/s2`,
				`s:parent/s1`,
				`s:parent`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/parent",
					"/parent/s1",
					"/parent/s2",
					"/parent/s3",
					"/stack",
				),
			},
		},
		{
			name: "after sub-stack of parent",
			layout: []string{
				`s:stack:after=["/parent/s2"]`,
				`s:parent/s3`,
				`s:parent/s2`,
				`s:parent/s1`,
				`s:parent`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/parent",
					"/parent/s1",
					"/parent/s2",
					"/parent/s3",
					"/stack",
				),
			},
		},
		{
			name: "before directory containing stacks",
			layout: []string{
				`d:dir`,
				`s:dir/s1`,
				`s:dir/s2`,
				`s:dir/s3`,
				`s:stack:before=["/dir"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack",
					"/dir/s1",
					"/dir/s2",
					"/dir/s3",
				),
			},
		},
		{
			name: "after directory containing stacks in deep directories",
			layout: []string{
				`d:dir`,
				`s:dir/A/B/C/D/Z-stack`,
				`s:A-stack:after=["/dir"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/dir/A/B/C/D/Z-stack",
					"/A-stack",
				),
			},
		},
		{
			name: "before directory containing no stacks does nothing",
			layout: []string{
				`d:dir`,
				`d:dir/dir2`,
				`s:stack:before=["/dir"]`,
				`s:stack2`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack",
					"/stack2",
				),
			},
		},
		{
			name: "after containing invalid tag filter",
			layout: []string{
				`s:stack:after=["tag:_invalid"]`,
				`s:stack2`,
			},
			want: runExpected{
				StderrRegex: string(tag.ErrInvalidTag),
				Status:      1,
			},
		},
		{
			name: "after directory containing no stacks does nothing",
			layout: []string{
				`d:dir`,
				`d:dir/dir2`,
				`s:stack:after=["/dir"]`,
				`s:stack2`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack",
					"/stack2",
				),
			},
		},
		{
			name: "stack-b after stack-a but filtered by tag:prod",
			layout: []string{
				`s:stack-a:tags=["dev"]`,
				`s:stack-b:tags=["prod"];after=["/stack-a"]`,
			},
			filterTags: []string{"prod"},
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
				),
			},
		},
		{
			name: "stack-b after stack-a but filtered by not having tag:dev",
			layout: []string{
				`s:stack-a:tags=["dev"]`,
				`s:stack-b:tags=["prod"];after=["/stack-a"]`,
			},
			filterNoTags: []string{"dev"},
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
				),
			},
		},
		{
			name: "combining --tags and --no-tags",
			layout: []string{
				`s:stack-a:tags=["a", "b", "c"]`,
				`s:stack-b:tags=["a", "b"]`,
			},
			filterTags:   []string{"a", "b"},
			filterNoTags: []string{"c"},
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
				),
			},
		},
		{
			name: "stack before unknown tag - ignored",
			layout: []string{
				`s:stack1`,
				`s:stack2`,
				`s:stack3:before=["tag:unknown"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack1",
					"/stack2",
					"/stack3",
				),
			},
		},
		{
			name: "stack3 before stacks with tag:core",
			layout: []string{
				`s:stack1:tags=["core"]`,
				`s:stack2`,
				`s:stack3:before=["tag:core"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack3",
					"/stack1",
					"/stack2",
				),
			},
		},
		{
			name: "stack2 before stacks with tag:core or tag:test",
			layout: []string{
				`s:stack1:tags=["core"]`,
				`s:stack2:before=["tag:core", "tag:test"]`,
				`s:stack3:tags=["test"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack2",
					"/stack1",
					"/stack3",
				),
			},
		},
		{
			name: "stacks ordered by tag multiple tag filters",
			layout: []string{
				`s:infra1:tags=["infra", "prod"]`,
				`s:infra2:tags=["infra", "prod"]`,
				`s:infra3:tags=["infra", "dev"]`,
				`s:k8s-infra1:tags=["k8s", "prod"];after=["tag:infra:prod"]`,
				`s:k8s-infra2:tags=["k8s", "dev"];after=["tag:infra:dev"]`,
				`s:app1:tags=["app"];after=["tag:k8s:prod"]`,
				`s:app2:tags=["app", "dev"];after=["tag:k8s:dev"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/infra1",
					"/infra2",
					"/k8s-infra1",
					"/app1",
					"/infra3",
					"/k8s-infra2",
					"/app2",
				),
			},
		},
		{
			name: "regression: stacks with same prefix as working dir but outside the fs branch",
			layout: []string{
				"s:stacks/test",
				"s:stacks/test-foo",
			},
			workingDir: "/stacks/test",
			want: runExpected{
				Stdout: listStacks(
					"/stacks/test",
				),
			},
		},
		{
			name: "regression: stacks with same prefix as working dir but outside the fs branch",
			layout: []string{
				"s:test",
				"s:tests",
			},
			workingDir: "/test",
			want: runExpected{
				Stdout: listStacks(
					"/test",
				),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sandboxes := []sandbox.S{
				sandbox.New(t),
				sandbox.NoGit(t),
			}

			for _, s := range sandboxes {
				s.BuildTree(tc.layout)
				test.WriteRootConfig(t, s.RootDir())

				wd := s.RootDir()
				if tc.workingDir != "" {
					wd = filepath.Join(wd, tc.workingDir)
				}

				var filterArgs []string
				for _, filter := range tc.filterTags {
					filterArgs = append(filterArgs, "--tags", filter)
				}
				for _, filter := range tc.filterNoTags {
					filterArgs = append(filterArgs, "--no-tags", filter)
				}

				cli := newCLI(t, wd)
				assertRunResult(t, cli.stacksRunOrder(filterArgs...), tc.want)
			}
		})
	}
}

func TestRunWants(t *testing.T) {
	t.Parallel()

	for _, tc := range []selectionTestcase{
		{
			/* this works but gives a warning */
			name: "stack-a wants stack-a",
			layout: []string{
				`s:stack-a:wants=["/stack-a"]`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
				),
			},
		},
		{
			name: "stack-a wants stack-b",
			layout: []string{
				`s:stack-a:wants=["/stack-b"]`,
				`s:stack-b`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-b wants stack-a (same ordering)",
			layout: []string{
				`s:stack-b:wants=["/stack-a"]`,
				`s:stack-a`,
			},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-a wants stack-b (from inside stack-a)",
			layout: []string{
				`s:stack-a:wants=["/stack-b"]`,
				`s:stack-b`,
			},
			wd: "/stack-a",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-a wants stack-b (from inside stack-b)",
			layout: []string{
				`s:stack-a:wants=["/stack-b"]`,
				`s:stack-b`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
				),
			},
		},
		{
			name: "stack-b wants stack-a (same ordering) (from inside stack-b)",
			layout: []string{
				`s:stack-b:wants=["/stack-a"]`,
				`s:stack-a`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
				),
			},
		},
		{
			name: "stack-b wants (stack-a, stack-c) (from inside stack-b)",
			layout: []string{
				`s:stack-b:wants=["/stack-a", "/stack-c"]`,
				`s:stack-a`,
				`s:stack-c`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
				),
			},
		},
		{
			name: "stack-b wants (stack-a, stack-c), stack-c wants stack-a (from inside stack-b)",
			layout: []string{
				`s:stack-b:wants=["/stack-a", "/stack-c"]`,
				`s:stack-a`,
				`s:stack-c:wants=["/stack-a"]`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
				),
			},
		},
		{
			name: `stack-a wants (stack-b, stack-c) and stack-b wants (stack-d, stack-e)
					(from inside stack-a) - recursive`,
			layout: []string{
				`s:stack-a:wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d`,
				`s:stack-e`,
			},
			wd: "/stack-a",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-e",
				),
			},
		},
		{
			name: `stack-a wants (stack-b, stack-c) and stack-b wants (stack-d, stack-e)
					(from inside stack-b) - not recursive`,
			layout: []string{
				`s:stack-a:wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d`,
				`s:stack-e`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
					"/stack-d",
					"/stack-e",
				),
			},
		},
		{
			/*
				stack-a wants (stack-b, stack-c)
					stack-b wants (stack-d, stack-e)
					stack-e wants (stack-a, stack-z)
					(from inside stack-b) - recursive, *circular*
					must pull all stacks`
			*/
			name: `must pull all stacks - recursive and circular`,
			layout: []string{
				`s:stack-a:wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d`,
				`s:stack-e:wants=["/stack-a", "/stack-z"]`,
				`s:stack-z`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-e",
					"/stack-z",
				),
			},
		},
		{
			/*
				  	wants+order - stack-a after stack-b / stack-d before stack-a
					stack-a wants (stack-b, stack-c)
					stack-b wants (stack-d, stack-e)
					stack-e wants (stack-a, stack-z) (from inside stack-b) - recursive, *circular*`
			*/
			name: `wants+ordering - recursive+circular`,
			layout: []string{
				`s:stack-a:wants=["/stack-b", "/stack-c"];after=["/stack-b"]`,
				`s:stack-b:wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d:before=["/stack-a"]`,
				`s:stack-e:wants=["/stack-a", "/stack-z"]`,
				`s:stack-z`,
			},
			wd: "/stack-b",
			want: runExpected{
				Stdout: listStacks(
					"/stack-b",
					"/stack-d",
					"/stack-a",
					"/stack-c",
					"/stack-e",
					"/stack-z",
				),
			},
		},
		{
			name: `stacks are filtered before wants/wanted by are computed`,
			layout: []string{
				`s:stack-a:tags=["infra"];wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:tags=["infra"];wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d:tags=["k8s"]`,
				`s:stack-e:tags=["k8s"]`,
			},
			wd:         "/stack-a",
			filterTags: []string{"k8s"},
			want: runExpected{
				Stdout: "",
			},
		},
		{
			name: `wants/wantedBy are not filtered`,
			layout: []string{
				`s:stack-a:tags=["k8s"];wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:tags=["k8s"];wants=["/stack-d", "/stack-e"]`,
				`s:stack-c`,
				`s:stack-d:tags=["infra"]`,
				`s:stack-e:tags=["infra"]`,
			},
			wd:         "/stack-a",
			filterTags: []string{"k8s"},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-e",
				),
			},
		},
		{
			name: `wants/wantedBy are not filtered by --no-tags`,
			layout: []string{
				`s:stack-a:tags=["k8s"];wants=["/stack-b", "/stack-c"]`,
				`s:stack-b:tags=["k8s"];wants=["/stack-d", "/stack-e"]`,
				`s:stack-c:tags=["infra", "k8s"]`,
				`s:stack-d:tags=["infra"]`,
				`s:stack-e:tags=["infra"]`,
			},
			wd:           "/stack-a",
			filterNoTags: []string{"infra"},
			want: runExpected{
				Stdout: listStacks(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-e",
				),
			},
		},
		{
			name: "stack-a wants with tag:query - fails",
			layout: []string{
				`s:stack-a:wants=["tag:prod"]`,
				`s:stack-b:tags=["prod"]`,
			},
			want: runExpected{
				Status:      1,
				StderrRegex: "filter is not allowed",
			},
		},
	} {
		testRunSelection(t, tc)
	}
}

func TestRunWantedBy(t *testing.T) {
	t.Parallel()

	for _, tc := range []selectionTestcase{
		{
			name: "stack wantedBy other-stack",
			layout: []string{
				`s:stack:wanted_by=["/other-stack"]`,
				`s:other-stack`,
			},
			wd: "/other-stack",
			want: runExpected{
				Stdout: listStacks(
					"/other-stack",
					"/stack",
				),
			},
		},
		{
			name: "stack1 wantedBy multiple stacks",
			layout: []string{
				`s:stack1:wanted_by=["/stack2", "/stack3"]`,
				`s:stack2`,
				`s:stack3`,
			},
			wd: "/stack2",
			want: runExpected{
				Stdout: listStacks(
					"/stack1",
					"/stack2",
				),
			},
		},
		{
			name: "stack1 (wants stack3) wantedBy stack2",
			layout: []string{
				`s:stack1:wanted_by=["/stack2"];wants=["/stack3"]`,
				`s:stack2`,
				`s:stack3`,
			},
			wd: "/stack2",
			want: runExpected{
				Stdout: listStacks(
					"/stack1",
					"/stack2",
					"/stack3",
				),
			},
		},
		{
			name: "stack1 wants stack3; stack1 is wantedBy stack2; stack3 wants stack4",
			layout: []string{
				`s:stack1:wanted_by=["/stack2"];wants=["/stack3"]`,
				`s:stack2`,
				`s:stack3:wants=["/stack4"]`,
				`s:stack4`,
			},
			wd: "/stack2",
			want: runExpected{
				Stdout: listStacks(
					"/stack1",
					"/stack2",
					"/stack3",
					"/stack4",
				),
			},
		},
		{
			name: "stack1 wanted_by stack2 and stack2 wanted_by stack1",
			layout: []string{
				`s:stack1:wanted_by=["/stack2"]`,
				`s:stack2:wanted_by=["/stack1"]`,
			},
			wd: "/stack1",
			want: runExpected{
				Stdout: listStacks(
					"/stack1",
					"/stack2",
				),
			},
		},
		{
			name: "stack wantedBy all other stacks - running 1",
			layout: []string{
				`s:stack:wanted_by=["/all"]`,
				`s:all/test/1`,
				`s:all/test2/2`,
				`s:all/something/3`,
			},
			wd: "/all/test/1",
			want: runExpected{
				Stdout: listStacks(
					"/all/test/1",
					"/stack",
				),
			},
		},
		{
			name: "stack wantedBy all other stacks - running 2",
			layout: []string{
				`s:stack:wanted_by=["/all"]`,
				`s:all/1`,
				`s:all/2`,
				`s:all/3`,
			},
			wd: "/all/2",
			want: runExpected{
				Stdout: listStacks(
					"/all/2",
					"/stack",
				),
			},
		},
		{
			name: "wantedBy different stacks already selected",
			layout: []string{
				`s:stack1:wanted_by=["/all/1"]`,
				`s:stack2:wanted_by=["/all/2"]`,
				`s:all/1`,
				`s:all/2`,
			},
			wd: "/all",
			want: runExpected{
				Stdout: listStacks(
					"/all/1",
					"/all/2",
					"/stack1",
					"/stack2",
				),
			},
		},
		{
			name: "wantedBy different stacks already selected with wants",
			layout: []string{
				`s:stack1:wanted_by=["/all/1"]`,
				`s:stack2:wanted_by=["/all/2"]`,
				`s:all/1:wants=["/all/2"]`,
				`s:all/2`,
			},
			wd: "/all/1",
			want: runExpected{
				Stdout: listStacks(
					"/all/1",
					"/all/2",
					"/stack1",
					"/stack2",
				),
			},
		},
		{
			name: "selected wantedBy stacks not filtered by tags",
			layout: []string{
				`s:stack:tags=["dev"];wanted_by=["/all"]`,
				`s:all/test/1:tags=["prod"]`,
				`s:all/test2/2`,
				`s:all/something/3`,
			},
			wd:         "/all/test/1",
			filterTags: []string{"prod"},
			want: runExpected{
				Stdout: listStacks(
					"/all/test/1",
					"/stack",
				),
			},
		},
		{
			name: "selected wantedBy stacks not filtered by --no-tags",
			layout: []string{
				`s:stack:tags=["dev"];wanted_by=["/all"]`,
				`s:all/test/1:tags=["prod"]`,
				`s:all/test2/2:tags=["dev"]`,
				`s:all/something/3:tags=["dev"]`,
			},
			wd:           "/all/test/1",
			filterNoTags: []string{"dev"},
			want: runExpected{
				Stdout: listStacks(
					"/all/test/1",
					"/stack",
				),
			},
		},
		{
			name: "wantedBy computed after stack filtering",
			layout: []string{
				`s:stack:tags=["prod"];wanted_by=["/other-stack"]`,
				`s:other-stack`,
			},
			wd:         "/other-stack",
			filterTags: []string{"prod"},
			want: runExpected{
				Stdout: "",
			},
		},
		{
			name: "wantedBy computed after stack filtering by --no-tags",
			layout: []string{
				`s:stack:tags=["prod"];wanted_by=["/other-stack"]`,
				`s:other-stack:tags=["abc"]`,
			},
			wd:           "/other-stack",
			filterNoTags: []string{"abc"},
			want: runExpected{
				Stdout: "",
			},
		},
		{
			name: "wantedBy computed after -C and tag filtering",
			layout: []string{
				`s:stacks/stack-a:tags=["dev"];wanted_by=["/other-stack"]`,
				`s:stacks/stack-b:tags=["prod"]`,
				`s:stack-c:tags=["prod"]`,
			},
			wd:         "/stacks",
			filterTags: []string{"prod"},
			want: runExpected{
				Stdout: listStacks(
					"/stacks/stack-b",
				),
			},
		},
		{
			name: "stack-a wanted_by with tag:query - fails",
			layout: []string{
				`s:stack-a:wanted_by=["tag:prod"]`,
				`s:stack-b:tags=["prod"]`,
			},
			want: runExpected{
				Status:      1,
				StderrRegex: "filter is not allowed",
			},
		},
		{
			name: "is not permitted to use internal filter syntax",
			layout: []string{
				`s:stack-a:tags=["prod"]`,
			},
			filterTags: []string{
				"~dev",
			},
			want: runExpected{
				Status:      1,
				StderrRegex: string(tag.ErrInvalidTag),
			},
		},
		{
			name: "is not permitted to use internal filter syntax - advanced case",
			layout: []string{
				`s:stack-a:tags=["prod"]`,
			},
			filterTags: []string{
				"prod:~experimental",
			},
			want: runExpected{
				Status:      1,
				StderrRegex: string(tag.ErrInvalidTag),
			},
		},
	} {
		testRunSelection(t, tc)
	}
}

func testRunSelection(t *testing.T, tc selectionTestcase) {
	sandboxes := []sandbox.S{
		sandbox.New(t),
		sandbox.NoGit(t),
	}

	for _, s := range sandboxes {
		s := s
		t.Run(tc.name, func(t *testing.T) {
			s.BuildTree(tc.layout)
			test.WriteRootConfig(t, s.RootDir())

			var baseArgs []string
			for _, filter := range tc.filterTags {
				baseArgs = append(baseArgs, "--tags", filter)
			}
			for _, filter := range tc.filterNoTags {
				baseArgs = append(baseArgs, "--no-tags", filter)
			}

			cli := newCLI(t, filepath.Join(s.RootDir(), tc.wd))

			runOrderArgs := append(baseArgs, "experimental", "run-order")
			assertRunResult(t, cli.run(runOrderArgs...), tc.want)

			if s.IsGit() {
				// required because `terramate run` requires a clean repo.
				git := s.Git()
				git.CommitAll("everything")
			}

			runArgs := append(baseArgs, "run", testHelperBin, "stack-abs-path", s.RootDir())
			assertRunResult(t, cli.run(runArgs...), tc.want)
		})
	}
}
func TestRunOrderNotChangedStackIgnored(t *testing.T) {
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

	cat := test.LookPath(t, "cat")
	wantRun := mainTfContents

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: wantRun})

	cli = newCLI(t, stack.Path())
	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: wantRun})

	cli = newCLI(t, filepath.Join(s.RootDir(), "stack2"))
	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{})
}

func TestRunReverseExecution(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)
	cat := test.LookPath(t, "cat")
	cli := newCLI(t, s.RootDir())
	assertRunOrder := func(stacks ...string) {
		t.Helper()

		want := strings.Join(stacks, "\n")
		if want != "" {
			want += "\n"
		}

		assertRunResult(t, cli.run(
			"run",
			"--reverse",
			cat,
			testfile,
		), runExpected{Stdout: want})

		assertRunResult(t, cli.run(
			"run",
			"--reverse",
			"--changed",
			cat,
			testfile,
		), runExpected{Stdout: want})
	}
	addStack := func(stack string) {
		s.BuildTree([]string{
			"s:" + stack,
			fmt.Sprintf("f:%s/%s:%s\n", stack, testfile, stack),
		})
	}

	git := s.Git()
	git.CheckoutNew("changes")

	assertRunOrder()

	addStack("stack-1")
	git.CommitAll("commit")
	assertRunOrder("stack-1")

	addStack("stack-2")
	git.CommitAll("commit")
	assertRunOrder("stack-2", "stack-1")

	addStack("stack-3")
	git.CommitAll("commit")
	assertRunOrder("stack-3", "stack-2", "stack-1")
}

func TestRunIgnoresAfterBeforeStackRefsOutsideWorkingDirAndTagFilter(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)

	s.BuildTree([]string{
		`s:parent-stack:tags=[]`,
		`s:stacks/stack-1:tags=["stack-1"];before=["/parent-stack"]`,
		`s:stacks/stack-2:tags=["stack-2"];after=["/parent-stack"]`,
		fmt.Sprintf("f:parent-stack/%s:parent-stack\n", testfile),
		fmt.Sprintf("f:stacks/stack-1/%s:stack-1\n", testfile),
		fmt.Sprintf("f:stacks/stack-2/%s:stack-2\n", testfile),
	})

	git := s.Git()
	git.CommitAll("first commit")

	assertRun := func(wd string, filter string, want string) {
		cli := newCLI(t, filepath.Join(s.RootDir(), wd))
		var baseArgs []string
		if filter != "" {
			baseArgs = append(baseArgs, "--tags", filter)
		}
		runArgs := append(baseArgs, "run", testHelperBin, "cat", testfile)
		assertRunResult(t, cli.run(runArgs...), runExpected{Stdout: want})

		runChangedArgs := append(baseArgs, "run",
			"--changed",
			testHelperBin,
			"cat",
			testfile,
		)

		assertRunResult(t, cli.run(runChangedArgs...), runExpected{Stdout: want})
	}

	assertRun(".", "", listStacks("stack-1", "parent-stack", "stack-2"))
	assertRun(".", "stack-1", listStacks("stack-1"))
	assertRun("stacks", "stack-1,stack-2", listStacks("stack-1", "stack-2"))
	assertRun("stacks", "stack-2", listStacks("stack-2"))
	assertRun("stacks/stack-1", "", listStacks("stack-1"))
	assertRun("stacks/stack-2", "", listStacks("stack-2"))
}

func TestRunOrderAllChangedStacksExecuted(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	// stack2 must run after stack and both changed.
	s.BuildTree([]string{
		`s:stack:after=["/stack2"]`,
		"s:stack2",
	})

	stack2 := s.DirEntry("stack2")
	stack2MainTf := stack2.CreateFile(mainTfFileName, "# some code")

	stack := s.DirEntry("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	stack2MainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	cli := newCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n" + stack2.RelPath() + "\n"
	assertRunResult(t, cli.listChangedStacks(), runExpected{Stdout: wantList})

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

func TestRunFailIfGitSafeguardUntracked(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# some code"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	stack.CreateFile(mainTfFileName, mainTfContents)

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stack.CreateFile("untracked-file.txt", `# something`)

	cli := newCLI(t, s.RootDir())
	cat := test.LookPath(t, "cat")

	// check untracked with --changed
	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has untracked files",
	})

	// check untracked *without* --changed
	assertRunResult(t, cli.run(
		"run",
		cat,
		mainTfFileName,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has untracked files",
	})

	// disabling the check must work for both with and without --changed

	t.Run("disable check using cmd args", func(t *testing.T) {
		assertRun(t, cli.run(
			"run",
			"--changed",
			"--disable-check-git-untracked",
			cat,
			mainTfFileName,
		))

		assertRunResult(t, cli.run(
			"run",
			"--disable-check-git-untracked",
			cat,
			mainTfFileName,
		), runExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using env vars", func(t *testing.T) {
		cli := newCLI(t, s.RootDir())
		cli.env = append([]string{
			"TM_DISABLE_CHECK_GIT_UNTRACKED=true",
		}, testEnviron(t)...)
		assertRun(t, cli.run(
			"run",
			"--changed",
			cat,
			mainTfFileName,
		))

		assertRunResult(t, cli.run(
			"run",
			cat,
			mainTfFileName,
		), runExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using hcl config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_untracked = false
			    }
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		assertRun(t, cli.run(
			"run",
			"--changed",
			cat,
			mainTfFileName,
		))

		assertRunResult(t, cli.run(
			"run",
			cat,
			mainTfFileName,
		), runExpected{
			Stdout: mainTfContents,
		})
	})
}

func TestRunFailIfOrphanedGenCodeIsDetected(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.CreateStack("stack")
	orphanEntry := s.CreateStack("orphan")
	orphanEntry.CreateFile("config.tm", GenerateHCL(
		Labels("test.tf"),
		Content(
			Str("test", "test"),
		),
	).String())

	tmcli := newCLI(t, s.RootDir())
	assertRunResult(t, tmcli.run("generate"), runExpected{
		IgnoreStdout: true,
	})

	git := s.Git()
	git.CommitAll("generated code")

	assertRunResult(t, tmcli.run(
		"run",
		testHelperBin,
		"env",
	), runExpected{
		IgnoreStdout: true,
	})

	orphanEntry.RemoveFile("config.tm")
	orphanEntry.DeleteStackConfig()

	git.CommitAll("deleted stack")

	assertRunResult(t, tmcli.run(
		"run",
		testHelperBin,
		"env",
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})
}

func TestRunFailIfGeneratedCodeIsOutdated(t *testing.T) {
	t.Parallel()

	const generateFile = "generate.tm.hcl"

	s := sandbox.New(t)
	stack := s.CreateStack("stack")

	// So we can list the stack as changed
	stack.CreateFile(generateFile, "")
	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("generate-code")

	generateFileBody := GenerateHCL(
		Labels("test.tf"),
		Content(
			Str("test", "test"),
		),
	).String()
	stack.CreateFile(generateFile, generateFileBody)

	git.CommitAll("generated code")

	tmcli := newCLI(t, s.RootDir())

	// check with --changed
	assertRunResult(t, tmcli.run(
		"run",
		"--changed",
		testHelperBin,
		"cat",
		generateFile,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// check without --changed
	assertRunResult(t, tmcli.run(
		"run",
		testHelperBin,
		"cat",
		generateFile,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// disabling the check must work for both with and without --changed

	t.Run("disable check using cmd args", func(t *testing.T) {
		assertRunResult(t, tmcli.run(
			"run",
			"--changed",
			"--disable-check-gen-code",
			testHelperBin,
			"cat",
			generateFile,
		), runExpected{
			Stdout: generateFileBody,
		})

		assertRunResult(t, tmcli.run(
			"run",
			"--disable-check-gen-code",
			testHelperBin,
			"cat",
			generateFile,
		), runExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using env vars", func(t *testing.T) {
		tmcli := newCLI(t, s.RootDir())
		tmcli.env = append([]string{
			"TM_DISABLE_CHECK_GEN_CODE=true",
		}, testEnviron(t)...)

		assertRunResult(t, tmcli.run("run", "--changed", testHelperBin, "cat", generateFile), runExpected{
			Stdout: generateFileBody,
		})
		assertRunResult(t, tmcli.run("run", testHelperBin, "cat", generateFile), runExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using hcl config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    run {
			      check_gen_code = false
			    }
			  }
			}
		`)

		git.Add(rootConfig)
		git.Commit("commit root config")

		assertRunResult(t, tmcli.run("run", "--changed", testHelperBin, "cat", generateFile), runExpected{
			Stdout: generateFileBody,
		})
		assertRunResult(t, tmcli.run("run", testHelperBin, "cat", generateFile), runExpected{
			Stdout: generateFileBody,
		})
	})
}

func TestRunFailIfGitSafeguardUncommitted(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName        = "main.tf"
		mainTfInitialContents = "# some code"
		mainTfAlteredContents = "# other code"
	)

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	file := stack.CreateFile(mainTfFileName, mainTfInitialContents)

	git := s.Git()
	git.CommitAll("first commit")

	cli := newCLI(t, s.RootDir())
	cat := test.LookPath(t, "cat")

	// everything committed, repo is clean
	assertRunResult(t, cli.run(
		"run",
		cat,
		mainTfFileName,
	), runExpected{Stdout: mainTfInitialContents})

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{Stdout: mainTfInitialContents})

	// make it uncommitted
	file.Write(mainTfAlteredContents)

	assertRunResult(t, cli.run(
		"run",
		cat,
		mainTfFileName,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has uncommitted files",
	})

	assertRunResult(t, cli.run(
		"run",
		"--changed",
		cat,
		mainTfFileName,
	), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has uncommitted files",
	})

	// --dry-run ignore safeguards
	assertRunResult(t, cli.run(
		"run",
		"--dry-run",
		cat,
		mainTfFileName,
	), runExpected{
		IgnoreStdout: true,
	})

	// disable uncommitted check

	t.Run("disable check using cmd args", func(t *testing.T) {
		assertRunResult(t, cli.run(
			"--disable-check-git-uncommitted",
			"run",
			cat,
			mainTfFileName,
		), runExpected{
			Stdout: mainTfAlteredContents,
		})

		assertRunResult(t, cli.run(
			"--disable-check-git-uncommitted",
			"--changed",
			"run",
			cat,
			mainTfFileName,
		), runExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("disable check using env vars", func(t *testing.T) {
		cli := newCLI(t, s.RootDir())
		cli.env = append([]string{
			"TM_DISABLE_CHECK_GIT_UNCOMMITTED=true",
		}, testEnviron(t)...)

		assertRunResult(t, cli.run("run", cat, mainTfFileName), runExpected{
			Stdout: mainTfAlteredContents,
		})
		assertRunResult(t, cli.run("--changed", "run", cat, mainTfFileName), runExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("disable check using hcl config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_uncommitted = false
			    }
			  }
			}
		`)

		git.Add(rootConfig)
		git.Commit("commit root config")

		assertRunResult(t, cli.run("run", cat, mainTfFileName), runExpected{
			Stdout: mainTfAlteredContents,
		})
		assertRunResult(t, cli.run("--changed", "run", cat, mainTfFileName), runExpected{
			Stdout: mainTfAlteredContents,
		})
	})
}

func TestRunFailIfStackGeneratedCodeIsOutdated(t *testing.T) {
	t.Parallel()

	const (
		testFilename   = "test.txt"
		contentsStack1 = "stack-1 file"
		contentsStack2 = "stack-2 file"
	)
	s := sandbox.New(t)

	stack1 := s.CreateStack("stacks/stack-1")
	stack2 := s.CreateStack("stacks/stack-2")

	stack1.CreateFile(testFilename, contentsStack1)
	stack2.CreateFile(testFilename, contentsStack2)

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")

	tmcli := newCLI(t, s.RootDir())
	cat := test.LookPath(t, "cat")

	assertRunResult(t, tmcli.run("run", cat, testFilename), runExpected{
		Stdout: contentsStack1 + contentsStack2,
	})

	stack1.CreateConfig(`
		generate_hcl "test.tf" {
		  content {
		    test = terramate.path
		  }
		}
	`)

	git.CheckoutNew("adding-stack1-config")
	git.CommitAll("adding stack-1 config")

	assertRunResult(t, tmcli.run("run", cat, testFilename), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	assertRunResult(t, tmcli.run("run", "--changed", cat, testFilename), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// Check that if inside cwd it should still detect changes outside
	tmcli = newCLI(t, stack2.Path())

	assertRunResult(t, tmcli.run("run", cat, testFilename), runExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})
}

func TestRunLogsUserCommand(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	stack := s.CreateStack("stack")
	testfile := stack.CreateFile("test", "")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")

	cli := newCLIWithLogLevel(t, s.RootDir(), "info")
	assertRunResult(t, cli.run("run", testHelperBin, "cat", testfile.HostPath()), runExpected{
		StderrRegex: `cmd=`,
	})
}

func TestRunContinueOnError(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:s1`,
		`s:s2`,
	})

	const expectedOutput = "# no code"

	s2 := s.StackEntry("s2")
	s2.CreateFile("main.tf", expectedOutput)

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")

	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.run("run", "cat", "main.tf"), runExpected{
		StderrRegex: "one or more commands failed",
		Status:      1,
	})

	assertRunResult(t, cli.run("run", "--continue-on-error", "cat", "main.tf"), runExpected{
		IgnoreStderr: true,
		Stdout:       expectedOutput,
		Status:       1,
	})
}

func TestRunNoRecursive(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	s.BuildTree([]string{
		`f:file.txt:root`,
		`s:parent`,
		`s:parent/child1`,
		`s:parent/child2`,
	})

	parent := s.StackEntry("parent")
	parent.CreateFile("file.txt", "parent")

	child1 := s.StackEntry("parent/child1")
	child1.CreateFile("file.txt", "child1")

	child2 := s.StackEntry("parent/child2")
	child2.CreateFile("file.txt", "child2")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")

	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.run("run", "cat", "file.txt"), runExpected{
		Stdout: `parentchild1child2`,
	})

	cli = newCLI(t, parent.Path())
	assertRunResult(t, cli.run("run", "--no-recursive", "cat", "file.txt"),
		runExpected{
			Stdout: `parent`,
		},
	)

	cli = newCLI(t, child1.Path())
	assertRunResult(t, cli.run("run", "--no-recursive", "cat", "file.txt"),
		runExpected{
			Stdout: `child1`,
		},
	)

	cli = newCLI(t, s.RootDir())
	assertRunResult(t, cli.run("run", "--no-recursive", "cat", "file.txt"),
		runExpected{
			Status:       1,
			IgnoreStderr: true,
		},
	)
}

func TestRunWitCustomizedEnv(t *testing.T) {
	t.Parallel()

	run := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("run", builders...)
	}
	env := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("env", builders...)
	}

	const (
		stackName             = "stack"
		stackGlobal           = "global from stack"
		exportedTerramateTest = "set on terramate test process"
		newTerramateOverriden = "newValue"
	)

	s := sandbox.New(t)

	root := s.RootEntry()
	stack := s.CreateStack(stackName)

	root.CreateFile("env.tm",
		Terramate(
			Config(
				run(
					env(
						Expr("FROM_META", "terramate.stack.name"),
						Expr("FROM_GLOBAL", "global.env"),
						Expr("FROM_ENV", "env.TERRAMATE_TEST"),
						Str("TERRAMATE_OVERRIDDEN", newTerramateOverriden),
					),
				),
			),
		).String(),
	)
	stack.CreateFile("globals.tm", Globals(
		Str("env", stackGlobal),
	).String())

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	hostenv := testEnviron(t)
	clienv := append(hostenv,
		"TERRAMATE_OVERRIDDEN=oldValue",
		fmt.Sprintf("TERRAMATE_TEST=%s", exportedTerramateTest),
	)

	tm := newCLI(t, s.RootDir())
	tm.env = clienv

	res := tm.run("run", testHelperBin, "env")
	if res.Status != 0 {
		t.Errorf("unexpected status code %d", res.Status)
		t.Logf("stdout:\n%s", res.Stdout)
		t.Logf("stderr:\n%s", res.Stderr)
		return
	}

	wantenv := append(hostenv,
		"CHECKPOINT_DISABLE=1", // e2e tests have telemetry disabled
		fmt.Sprintf("FROM_META=%s", stackName),
		fmt.Sprintf("FROM_GLOBAL=%s", stackGlobal),
		fmt.Sprintf("FROM_ENV=%s", exportedTerramateTest),
		fmt.Sprintf("TERRAMATE_TEST=%s", exportedTerramateTest),
		fmt.Sprintf("TERRAMATE_OVERRIDDEN=%s", newTerramateOverriden),
	)
	gotenv := strings.Split(strings.Trim(res.Stdout, "\n"), "\n")

	sort.Strings(gotenv)
	sort.Strings(wantenv)

	test.AssertDiff(t, gotenv, wantenv)

	t.Run("ExperimentalRunEnv", func(t *testing.T) {
		want := fmt.Sprintf(`
stack "/stack":
	FROM_ENV=%s
	FROM_GLOBAL=%s
	FROM_META=%s
	TERRAMATE_OVERRIDDEN=%s
`, exportedTerramateTest, stackGlobal, stackName, newTerramateOverriden)

		assertRunResult(t, tm.run("experimental", "run-env"), runExpected{
			Stdout: want})
	})
}

func listStacks(stacks ...string) string {
	return strings.Join(stacks, "\n") + "\n"
}

func testEnviron(t *testing.T) []string {
	tempHomeDir := t.TempDir()
	env := []string{
		fmt.Sprintf("%s="+tempHomeDir, cliconfig.DirEnv),
		"PATH=" + os.Getenv("PATH"),
	}
	if runtime.GOOS == "windows" {
		// https://pkg.go.dev/os/exec
		// As a special case on Windows, SYSTEMROOT is always added if
		// missing and not explicitly set to the empty string.
		env = append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	return env
}

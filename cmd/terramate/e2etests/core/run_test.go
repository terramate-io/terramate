// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

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
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

const defaultErrExitStatus = 1

type selectionTestcase struct {
	name         string
	layout       []string
	wd           string
	filterTags   []string
	filterNoTags []string
	want         RunExpected
}

func TestCLIRunOrder(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name         string
		layout       []string
		filterTags   []string
		filterNoTags []string
		workingDir   string
		want         RunExpected
	}

	for _, tc := range []testcase{
		{
			name: "one stack",
			layout: []string{
				"s:stack-a",
			},
			want: RunExpected{
				Stdout: nljoin(
					"/stack-a",
				),
			},
		},
		{
			name: "empty ordering",
			layout: []string{
				"s:stack:after=[]",
			},
			want: RunExpected{
				Stdout: nljoin(
					`/stack`,
				),
			},
		},
		{
			name: "after non-existent path",
			layout: []string{
				fmt.Sprintf("s:stack:after=[%q]", test.NonExistingDir(t)),
			},
			want: RunExpected{
				Stdout: nljoin(
					`/stack`,
				),
				StderrRegex: "Warning: Stack references invalid path in 'after' attribute",
			},
		},
		{
			name: "after regular file",
			layout: []string{
				fmt.Sprintf("s:stack:after=[%q]", test.WriteFile(t, "", "test.txt", `bleh`)),
			},
			want: RunExpected{
				Stdout: nljoin(
					`/stack`,
				),
				StderrRegex: "Warning: Stack references invalid path in 'after' attribute",
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Status:      defaultErrExitStatus,
				StderrRegex: string(dag.ErrCycleDetected),
			},
		},
		{
			name: "stack-a after . - fails",
			layout: []string{
				`s:stack-a:after=["."]`,
			},
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
					"/parent",
					"/parent/stack-a",
					"/parent/stack-b",
				),
			},
		},
		{
			name: "implicit order with tags - Zied case",
			layout: []string{
				`s:project:tags=["project"];before=["tag:identity"]`,
				`s:iac/cloud-storage/bucket:tags=["bucket"];after=["tag:project", "tag:service-account"]`,
				`s:iac/service-accounts:tags=["identity"];before=["/iac/service-accounts/sa-name"]`,
				`s:iac/service-accounts/sa-name:tags=["service-account"]`,
			},
			want: RunExpected{
				Stdout: nljoin(
					"/project",
					"/iac/service-accounts",
					"/iac/service-accounts/sa-name",
					"/iac/cloud-storage/bucket",
				),
			},
		},
		{
			name: "before clause pulling a branch of fs ordered stacks",
			layout: []string{
				`s:project:tags=["project"];before=["tag:parent"]`,
				`s:dir/parent:tags=["parent"]`,
				`s:dir/parent/child:tags=["child"]`,
				`s:dir/other:tags=["other"];after=["tag:project", "tag:child"]`,
			},
			want: RunExpected{
				Stdout: nljoin(
					`/project`,
					`/dir/parent`,
					`/dir/parent/child`,
					`/dir/other`,
				),
			},
		},
		{
			name: "stack pulled to the middle of fs ordering chain",
			layout: []string{
				`s:parent`,
				`s:parent/child:before=["tag:other"]`,
				`s:parent/child/grand-child`,
				`s:other:tags=["other"]`,
			},
			want: RunExpected{
				Stdout: nljoin(
					`/parent`,
					`/parent/child`,
					`/other`,
					`/parent/child/grand-child`,
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
					"/test",
				),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sandboxes := []sandbox.S{
				sandbox.New(t),
				sandbox.NoGit(t, true),
			}

			for _, s := range sandboxes {
				s := s
				t.Run("run on sandbox", func(t *testing.T) {
					t.Parallel()
					copiedLayout := make([]string, len(tc.layout))
					copy(copiedLayout, tc.layout)
					if runtime.GOOS != "windows" {
						copiedLayout = append(copiedLayout,

							fmt.Sprintf(`file:script.tm:
terramate {
	config {
		experiments = ["scripts"]
	}
}
script "cmd" {
	description = "test"
	job {
		command = ["%s", "stack-abs-path", "%s"]
	}
}`, HelperPath, s.RootDir()))
					}
					s.BuildTree(copiedLayout)

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

					cli := NewCLI(t, wd)
					AssertRunResult(t, cli.StacksRunOrder(filterArgs...), tc.want)
					runArgs := []string{
						"--quiet", "run", "-X", // disable all safeguards
					}
					runArgs = append(runArgs, filterArgs...)
					runArgs = append(runArgs, "--", HelperPath, "stack-abs-path", s.RootDir())
					AssertRunResult(t, cli.Run(runArgs...), tc.want)

					if runtime.GOOS != "windows" {
						runScriptArgs := []string{
							"--quiet",
						}
						runScriptArgs = append(runScriptArgs, filterArgs...)
						runScriptArgs = append(runScriptArgs, "script", "run", "-X", "cmd") // disable all safeguards)
						AssertRunResult(t, cli.Run(runScriptArgs...), RunExpected{
							Status:        tc.want.Status,
							IgnoreStderr:  true,
							Stdout:        tc.want.Stdout,
							IgnoreStdout:  tc.want.IgnoreStdout,
							StdoutRegex:   tc.want.StdoutRegex,
							StdoutRegexes: tc.want.StdoutRegexes,
						})
					}
				})
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
			want: RunExpected{
				Stdout: nljoin(
					"/stack-a",
				),
				StderrRegexes: []string{
					"Stack selection clauses \\(wants\\/wanted_by\\) have cycles",
					`cycle detected: /stack-a -> /stack-a: checking node id "/stack-a"`,
				},
			},
		},
		{
			name: "stack-a wants stack-b",
			layout: []string{
				`s:stack-a:wants=["/stack-b"]`,
				`s:stack-b`,
			},
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
					"/stack-a",
					"/stack-b",
					"/stack-c",
					"/stack-d",
					"/stack-e",
					"/stack-z",
				),
				StderrRegexes: []string{
					"Stack selection clauses \\(wants\\/wanted_by\\) have cycles",
					`cycle detected: /stack-a -> /stack-b -> /stack-e -> /stack-a: checking node id "/stack-a"`,
				},
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
			want: RunExpected{
				Stdout: nljoin(
					"/stack-b",
					"/stack-d",
					"/stack-a",
					"/stack-c",
					"/stack-e",
					"/stack-z",
				),
				StderrRegexes: []string{
					"Stack selection clauses \\(wants\\/wanted_by\\) have cycles",
					`cycle detected: /stack-a -> /stack-b -> /stack-e -> /stack-a: checking node id "/stack-a"`,
				},
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
					"/stack1",
					"/stack2",
				),
				StderrRegexes: []string{
					"Stack selection clauses \\(wants\\/wanted_by\\) have cycles",
					"cycle detected: /stack1 -> /stack2 -> /stack1: checking node id \"/stack1\"",
				},
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
				Stdout: nljoin(
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin(
					"/stacks/stack-b",
				),
				StderrRegexes: []string{
					"Stack references invalid path in 'wanted_by' attribute",
					"no such file or directory",
				},
			},
		},
		{
			name: "stack-a wanted_by with tag:query - fails",
			layout: []string{
				`s:stack-a:wanted_by=["tag:prod"]`,
				`s:stack-b:tags=["prod"]`,
			},
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
		sandbox.NoGit(t, true),
	}

	for _, s := range sandboxes {
		s := s
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			copiedLayout := make([]string, len(tc.layout))
			copy(copiedLayout, tc.layout)
			if runtime.GOOS != "windows" {
				copiedLayout = append(copiedLayout,

					fmt.Sprintf(`file:script.tm:
terramate {
	config {
		experiments = ["scripts"]
	}
}
script "cmd" {
	description = "test"
	job {
		command = ["%s", "stack-abs-path", "%s"]
	}
}`, HelperPath, s.RootDir()))
			}
			s.BuildTree(copiedLayout)

			var baseArgs []string
			for _, filter := range tc.filterTags {
				baseArgs = append(baseArgs, "--tags", filter)
			}
			for _, filter := range tc.filterNoTags {
				baseArgs = append(baseArgs, "--no-tags", filter)
			}

			cli := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))

			runOrderArgs := append(baseArgs, "--quiet", "experimental", "run-order")
			AssertRunResult(t, cli.Run(runOrderArgs...), tc.want)

			if s.IsGit() {
				// required because `terramate run` requires a clean repo.
				git := s.Git()
				git.CommitAll("everything")
			}

			copiedBaseArgs := make([]string, len(baseArgs))
			copy(copiedBaseArgs, baseArgs)
			runArgs := append(copiedBaseArgs, "run", "--quiet", HelperPath, "stack-abs-path", s.RootDir())
			AssertRunResult(t, cli.Run(runArgs...), tc.want)

			if runtime.GOOS != "windows" {
				scriptArgs := append(copiedBaseArgs, "script", "run", "cmd")
				AssertRunResult(t, cli.Run(scriptArgs...), RunExpected{
					Stdout:       tc.want.Stdout,
					IgnoreStderr: true,
					Status:       tc.want.Status,
				})
			}
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

	cli := NewCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: wantList})

	wantRun := mainTfContents

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: wantRun})

	cli = NewCLI(t, stack.Path())
	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: wantRun})

	cli = NewCLI(t, filepath.Join(s.RootDir(), "stack2"))
	AssertRunResult(t, cli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{})
}

func TestRunReverseExecution(t *testing.T) {
	t.Parallel()

	const testfile = "testfile"

	s := sandbox.New(t)
	cli := NewCLI(t, s.RootDir())
	assertRunOrder := func(stacks ...string) {
		t.Helper()

		want := strings.Join(stacks, "\n")
		if want != "" {
			want += "\n"
		}

		AssertRunResult(t, cli.Run(
			"run",
			"--quiet",
			"--reverse",
			HelperPath,
			"cat",
			testfile,
		), RunExpected{Stdout: want})

		AssertRunResult(t, cli.Run(
			"run",
			"--quiet",
			"--reverse",
			"--changed",
			HelperPath,
			"cat",
			testfile,
		), RunExpected{Stdout: want})
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
		cli := NewCLI(t, filepath.Join(s.RootDir(), wd))
		var baseArgs []string
		if filter != "" {
			baseArgs = append(baseArgs, "--tags", filter)
		}
		runArgs := append(baseArgs, "run", "--quiet", HelperPath, "cat", testfile)
		AssertRunResult(t, cli.Run(runArgs...), RunExpected{Stdout: want})

		runChangedArgs := append(baseArgs, "run",
			"--quiet",
			"--changed",
			HelperPath,
			"cat",
			testfile,
		)

		AssertRunResult(t, cli.Run(runChangedArgs...), RunExpected{Stdout: want})
	}

	assertRun(".", "", nljoin("stack-1", "parent-stack", "stack-2"))
	assertRun(".", "stack-1", nljoin("stack-1"))
	assertRun("stacks", "stack-1,stack-2", nljoin("stack-1", "stack-2"))
	assertRun("stacks", "stack-2", nljoin("stack-2"))
	assertRun("stacks/stack-1", "", nljoin("stack-1"))
	assertRun("stacks/stack-2", "", nljoin("stack-2"))
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

	cli := NewCLI(t, s.RootDir())

	wantList := stack.RelPath() + "\n" + stack2.RelPath() + "\n"
	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{
		Stdout: wantList,
	})

	wantRun := fmt.Sprintf(
		"%s%s",
		mainTfContents,
		mainTfContents,
	)

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: wantRun})
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

	t.Run("ensure test env is in the correct state", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir())

		// check untracked with --changed
		AssertRunResult(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
		})

		// check untracked *without* --changed
		AssertRunResult(t, tmcli.Run(
			"run",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
		})
	})

	t.Run("ensure list is not affected by untracked check", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir())

		AssertRun(t, tmcli.Run("list", "--changed"))
		AssertRunResult(t, tmcli.Run("list"), RunExpected{
			Stdout: nljoin("stack"),
		})
	})

	// disabling the check must work for both with and without --changed

	t.Run("disable check using deprecated cmd args", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			"--disable-check-git-untracked",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-check-git-untracked",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using --disable-safeguards=git-untracked cmd args", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--disable-safeguards=git-untracked",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"--quiet",
			"run",
			"--disable-safeguards=git-untracked",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using --disable-safeguards=all cmd args", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--disable-safeguards=all",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"--quiet",
			"run",
			"--disable-safeguards=all",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using -X", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"-X",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"--quiet",
			"run",
			"-X",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using env vars using env=true", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GIT_UNTRACKED=true")

		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using env vars using env=1", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GIT_UNTRACKED=1")

		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using terramate.config.git.check_untracked", func(t *testing.T) {
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

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using terramate.config.disable_safeguards=git-untracked", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["git-untracked"]
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("disable check using terramate.config.disable_safeguards=git", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["git"]
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("make sure --disable-safeguards=git-untracked has precedence over config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
			      check_untracked = true
			    }
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--disable-safeguards=git-untracked",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--disable-safeguards=git-untracked",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("make sure --disable-safeguards=git-untracked has precedence over disable_safeguards config", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["git-untracked"]
			  }
			}
		`)
		defer s.RootEntry().RemoveFile(rootConfig)

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRun(t, tmcli.Run(
			"run",
			"--disable-safeguards=git-untracked",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		))

		AssertRunResult(t, tmcli.Run(
			"run",
			"--disable-safeguards=git-untracked",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfContents,
		})
	})

	t.Run("make sure --disable-safeguards=none re-enables config option", func(t *testing.T) {
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

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		AssertRunResult(t, tmcli.Run(
			"run",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
		})
	})

	t.Run("make sure TM_DISABLE_SAFEGUARDS=none re-enables config option", func(t *testing.T) {
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

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=none")
		AssertRunResult(t, tmcli.Run(
			"run",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has untracked files",
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

	tmcli := NewCLI(t, s.RootDir())
	AssertRunResult(t, tmcli.Run("generate"), RunExpected{
		IgnoreStdout: true,
	})

	git := s.Git()
	git.CommitAll("generated code")

	AssertRunResult(t, tmcli.Run(
		"run",
		"--quiet",
		HelperPath,
		"env",
	), RunExpected{
		IgnoreStdout: true,
	})

	orphanEntry.RemoveFile("config.tm")
	orphanEntry.DeleteStackConfig()

	git.CommitAll("deleted stack")

	AssertRunResult(t, tmcli.Run(
		"run",
		HelperPath,
		"env",
	), RunExpected{
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

	tmcli := NewCLI(t, s.RootDir())

	// check with --changed
	AssertRunResult(t, tmcli.Run(
		"run",
		"--changed",
		HelperPath,
		"cat",
		generateFile,
	), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// check without --changed
	AssertRunResult(t, tmcli.Run(
		"run",
		HelperPath,
		"cat",
		generateFile,
	), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// disabling the check must work for both with and without --changed

	t.Run("disable check-gen-code using deprecated flags", func(t *testing.T) {
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--changed",
			"--disable-check-gen-code",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-check-gen-code",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable outdated-code check using --disable-safeguards=outdated-code", func(t *testing.T) {
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--changed",
			"--disable-safeguards=outdated-code",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=outdated-code",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable outdated-code check using -X", func(t *testing.T) {
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--changed",
			"-X",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"-X",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using env=true", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GEN_CODE=true")
		AssertRunResult(t, tmcli.Run("run", "--quiet", "--changed", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using env=1", func(t *testing.T) {
		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_CHECK_GEN_CODE=1")
		AssertRunResult(t, tmcli.Run("run", "--quiet", "--changed", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using terramate.config.run.check_gen_code", func(t *testing.T) {
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

		AssertRunResult(t, tmcli.Run("run", "--quiet", "--changed", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("disable check using terramate.config.disable_safeguards", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    disable_safeguards = ["outdated-code"]
			  }
			}
		`)

		git.Add(rootConfig)
		git.Commit("commit root config")

		AssertRunResult(t, tmcli.Run("run", "--quiet", "--changed", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
		AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath, "cat", generateFile), RunExpected{
			Stdout: generateFileBody,
		})
	})

	t.Run("re-enables check with --disable-safeguards=none", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    run {
				  # comment for file change
			      check_gen_code = false
			    }
			  }
			}
		`)

		git.Add(rootConfig)
		git.Commit("commit root config")

		AssertRunResult(t, tmcli.Run(
			"run",
			"--changed",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
		})
	})

	t.Run("re-enables check with TM_DISABLE_SAFEGUARDS=none", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    run {
				  # another comment for file change
			      check_gen_code = false
			    }
			  }
			}
		`)

		git.Add(rootConfig)
		git.Commit("commit root config")

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=none")
		AssertRunResult(t, tmcli.Run(
			"run",
			"--changed",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
		})

		AssertRunResult(t, tmcli.Run(
			"run",
			HelperPath,
			"cat",
			generateFile,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
		})
	})
}

func TestRunOutput(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name    string
		runArgs []string
		want    RunExpected
	}

	for _, tc := range []testcase{
		{
			name:    "run without eval",
			runArgs: []string{HelperPath, "echo", "hello"},
			want: RunExpected{
				Stderr: "terramate: Entering stack in /stack" + "\n" +
					fmt.Sprintf(`terramate: Executing command "%s echo hello"`, HelperPath) + "\n",
				Stdout: "hello\n",
			},
		},
		{
			name:    "run with eval",
			runArgs: []string{"--eval", HelperPath, "echo", "${terramate.stack.name}"},
			want: RunExpected{
				Stderr: "terramate: Entering stack in /stack" + "\n" +
					fmt.Sprintf(`terramate: Executing command "%s echo stack"`, HelperPath) + "\n",
				Stdout: "stack\n",
			},
		},
		{
			name:    "run with eval with error",
			runArgs: []string{"--eval", HelperPath, "echo", "${terramate.stack.abcabc}"},
			want: RunExpected{
				Stderr: "Error: unable to evaluate command" + "\n" +
					`> <cmd arg>:1,19-26: eval expression: eval "${terramate.stack.abcabc}": This object does not have an attribute named "abcabc"` + ".\n",
				Stdout: "",
				Status: 1,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			_ = s.CreateStack("stack")
			git := s.Git()
			git.CommitAll("first commit")
			cli := NewCLI(t, s.RootDir())
			AssertRunResult(t,
				cli.Run(append([]string{"run"}, tc.runArgs...)...),
				tc.want,
			)
		})
	}

}

func TestRunDryRun(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name    string
		runArgs []string
		want    RunExpected
	}

	for _, tc := range []testcase{
		{
			name:    "dryrun without eval",
			runArgs: []string{"--dry-run", HelperPath, "echo", "hello"},
			want: RunExpected{
				Stderr: "terramate: (dry-run) Entering stack in /stack" + "\n" +
					fmt.Sprintf(`terramate: (dry-run) Executing command "%s echo hello"`, HelperPath) + "\n",
				Stdout: "",
			},
		},
		{
			name:    "dryrun with eval",
			runArgs: []string{"--dry-run", "--eval", HelperPath, "echo", "${terramate.stack.name}"},
			want: RunExpected{
				Stderr: "terramate: (dry-run) Entering stack in /stack" + "\n" +
					fmt.Sprintf(`terramate: (dry-run) Executing command "%s echo stack"`, HelperPath) + "\n",
				Stdout: "",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			_ = s.CreateStack("stack")
			git := s.Git()
			git.CommitAll("first commit")
			cli := NewCLI(t, s.RootDir())
			AssertRunResult(t,
				cli.Run(append([]string{"run"}, tc.runArgs...)...),
				tc.want,
			)
		})
	}

}

func TestRunTerragrunt(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	_ = s.CreateStack("stack")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")

	// This just tests if the option is properly supported.
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		cli.Run("run", "--terragrunt", "--quiet", HelperPath, "true"),
		RunExpected{},
	)
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

	cli := NewCLI(t, s.RootDir())

	// everything committed, repo is clean
	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: mainTfInitialContents})

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{Stdout: mainTfInitialContents})

	// make it uncommitted
	file.Write(mainTfAlteredContents)

	AssertRunResult(t, cli.Run(
		"run",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has uncommitted files",
	})

	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--changed",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: "repository has uncommitted files",
	})

	// --dry-run ignore safeguards
	AssertRunResult(t, cli.Run(
		"run",
		"--quiet",
		"--dry-run",
		HelperPath,
		"cat",
		mainTfFileName,
	), RunExpected{
		IgnoreStdout: true,
	})

	// disable uncommitted check

	t.Run("disable check using deprecated args", func(t *testing.T) {
		AssertRunResult(t, cli.Run(
			"--disable-check-git-uncommitted",
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})

		AssertRunResult(t, cli.Run(
			"--disable-check-git-uncommitted",
			"--changed",
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("disable check using new --disable-safeguards=git-uncommitted args", func(t *testing.T) {
		AssertRunResult(t, cli.Run(
			"run",
			"--disable-safeguards=git-uncommitted",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})

		AssertRunResult(t, cli.Run(
			"--quiet",
			"--changed",
			"run",
			"--disable-safeguards=git-uncommitted",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("disable uncommitted safeguard using -X", func(t *testing.T) {
		AssertRunResult(t, cli.Run(
			"run",
			"-X",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})

		AssertRunResult(t, cli.Run(
			"--quiet",
			"--changed",
			"run",
			"-X",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("disable check using env vars", func(t *testing.T) {
		cli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		cli.AppendEnv = append(cli.AppendEnv, "TM_DISABLE_CHECK_GIT_UNCOMMITTED=true")

		AssertRunResult(t, cli.Run("run", "--quiet", HelperPath,
			"cat", mainTfFileName), RunExpected{
			Stdout: mainTfAlteredContents,
		})
		AssertRunResult(t, cli.Run("--changed", "run", "--quiet", HelperPath,
			"cat", mainTfFileName), RunExpected{
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

		AssertRunResult(t, cli.Run("run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName), RunExpected{
			Stdout: mainTfAlteredContents,
		})
		AssertRunResult(t, cli.Run(
			"--changed", "run", "--quiet", HelperPath,
			"cat", mainTfFileName), RunExpected{
			Stdout: mainTfAlteredContents,
		})
	})

	t.Run("re-enables check using --disable-safeguards=none", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
				  # this comment is needed to make the file change.
			      check_uncommitted = false
			    }
			  }
			}
		`)

		// TODO(i4k): this test is not isolated. Needs to be moved to safeguard_test.go
		git.Add(rootConfig)
		git.Commit("commit root config")

		AssertRunResult(t, cli.Run(
			"run",
			"--quiet",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has uncommitted files",
		})
		AssertRunResult(t, cli.Run(
			"run",
			"--quiet",
			"--changed",
			"--disable-safeguards=none",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has uncommitted files",
		})
	})

	t.Run("re-enables check using TM_DISABLE_SAFEGUARDS=none", func(t *testing.T) {
		const rootConfig = "terramate.tm.hcl"

		s.RootEntry().CreateFile(rootConfig, `
			terramate {
			  config {
			    git {
				  # another comment is needed to make the file change.
			      check_uncommitted = false
			    }
			  }
			}
		`)

		// TODO(i4k): this test is not isolated. Needs to be moved to safeguard_test.go
		git.Add(rootConfig)
		git.Commit("commit root config")

		tmcli := NewCLI(t, s.RootDir(), testEnviron(t)...)
		tmcli.AppendEnv = append(tmcli.AppendEnv, "TM_DISABLE_SAFEGUARDS=none")

		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has uncommitted files",
		})
		AssertRunResult(t, tmcli.Run(
			"run",
			"--quiet",
			"--changed",
			HelperPath,
			"cat",
			mainTfFileName,
		), RunExpected{
			Status:      defaultErrExitStatus,
			StderrRegex: "repository has uncommitted files",
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

	tmcli := NewCLI(t, s.RootDir())

	AssertRunResult(t, tmcli.Run("run", "--quiet", HelperPath,
		"cat", testFilename), RunExpected{
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

	AssertRunResult(t, tmcli.Run("run", HelperPath,
		"cat", testFilename), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	AssertRunResult(t, tmcli.Run("run", "--changed", HelperPath,
		"cat", testFilename), RunExpected{
		Status:      defaultErrExitStatus,
		StderrRegex: string(cli.ErrOutdatedGenCodeDetected),
	})

	// Check that if inside cwd it should still detect changes outside
	tmcli = NewCLI(t, stack2.Path())

	AssertRunResult(t, tmcli.Run("run", HelperPath,
		"cat", testFilename), RunExpected{
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

	cli := NewCLI(t, s.RootDir())
	cli.LogLevel = "info"
	AssertRunResult(t, cli.Run("run", HelperPath, "cat", testfile.HostPath()), RunExpected{
		StderrRegex: `Executing command`,
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

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("run", "--quiet", HelperPath,
		"cat", "main.tf"), RunExpected{
		StderrRegex: "one or more commands failed",
		Status:      1,
	})

	AssertRunResult(t, cli.Run("run", "--quiet", "--continue-on-error", HelperPath,
		"cat", "main.tf"), RunExpected{
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

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("run", "--quiet", HelperPath,
		"cat", "file.txt"), RunExpected{
		Stdout: `parentchild1child2`,
	})

	cli = NewCLI(t, parent.Path())
	AssertRunResult(t, cli.Run("run", "--quiet", "--no-recursive", HelperPath,
		"cat", "file.txt"),
		RunExpected{
			Stdout: `parent`,
		},
	)

	cli = NewCLI(t, child1.Path())
	AssertRunResult(t, cli.Run("run", "--quiet", "--no-recursive", HelperPath,
		"cat", "file.txt"),
		RunExpected{
			Stdout: `child1`,
		},
	)

	cli = NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("run", "--quiet", "--no-recursive", HelperPath,
		"cat", "file.txt"),
		RunExpected{
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

	tm := NewCLI(t, s.RootDir(), clienv...)

	res := tm.Run("run", HelperPath, "env")
	if res.Status != 0 {
		t.Errorf("unexpected status code %d", res.Status)
		t.Logf("stdout:\n%s", res.Stdout)
		t.Logf("stderr:\n%s", res.Stderr)
		return
	}

	wantenv := append(hostenv,
		"ACTIONS_ID_TOKEN_REQUEST_URL=",
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN=",
		"CHECKPOINT_DISABLE=1", // e2e tests have telemetry disabled
		fmt.Sprintf("FROM_META=%s", stackName),
		fmt.Sprintf("FROM_GLOBAL=%s", stackGlobal),
		fmt.Sprintf("FROM_ENV=%s", exportedTerramateTest),
		fmt.Sprintf("TERRAMATE_TEST=%s", exportedTerramateTest),
		fmt.Sprintf("TERRAMATE_OVERRIDDEN=%s", newTerramateOverriden),
	)
	gotenv := strings.Split(strings.Trim(res.Stdout, "\n"), "\n")

	// remove the custom cli config file.
	for i, e := range gotenv {
		if strings.HasPrefix(e, "TM_CLI_CONFIG_FILE=") {
			gotenv = append(gotenv[:i], gotenv[i+1:]...)
		}
	}

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

		AssertRunResult(t, tm.Run("debug", "show", "runtime-env"), RunExpected{
			IgnoreStderr: true,
			Stdout:       want})
	})
}

func nljoin(stacks ...string) string {
	return strings.Join(stacks, "\n") + "\n"
}

func testEnviron(t *testing.T) []string {
	tempHomeDir := test.TempDir(t)
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

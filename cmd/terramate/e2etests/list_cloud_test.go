// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver"
	cloudtest "github.com/terramate-io/terramate/test/cloud"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudListUnhealthy(t *testing.T) {
	type testcase struct {
		name       string
		layout     []string
		repository string
		stacks     []cloud.StackResponse
		flags      []string
		workingDir string
		want       runExpected
	}

	startFakeTMCServer(t)

	for _, tc := range []testcase{
		{
			name:       "only unhealthy filter is permitted",
			layout:     []string{"s:s1:id=s1"},
			repository: t.TempDir(),
			flags:      []string{`--experimental-status=drifted`},
			want: runExpected{
				Status:      1,
				StderrRegex: "only unhealthy filter allowed",
			},
		},
		{
			name:       "local repository is not permitted with --experimental-status=unhealthy",
			layout:     []string{"s:s1:id=s1"},
			repository: t.TempDir(),
			flags:      []string{`--experimental-status=unhealthy`},
			want: runExpected{
				Status:      1,
				StderrRegex: "unhealthy status filter does not work with filesystem based remotes",
			},
		},
		{
			name: "no cloud stacks, no status flag, return local stacks",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			want: runExpected{
				Stdout: nljoin("s1", "s2"),
			},
		},
		{
			name: "no cloud stacks, asking for unhealthy, return nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			flags: []string{"--experimental-status=unhealthy"},
		},
		{
			name: "1 cloud stack healthy, other absent, return nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.OK,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
		},
		{
			name: "1 cloud stack unhealthy but different repository, return nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "gitlab.com/unknown-io/other",
					},
					Status: stack.Failed,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
		},
		{
			name: "1 cloud stack unhealthy, other absent, return unhealthy",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Failed,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: runExpected{
				Stdout: nljoin("s1"),
			},
		},
		{
			name: "1 cloud stack unhealthy, other ok, return unhealthy",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Failed,
				},
				{
					ID: 2,
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.OK,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: runExpected{
				Stdout: nljoin("s1"),
			},
		},
		{
			name:   "no local stacks, 2 unhealthy stacks, return nothing",
			layout: []string{},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Failed,
				},
				{
					ID: 2,
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Drifted,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
		},
		{
			name: "2 local stacks, 2 same unhealthy stacks, return both",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Failed,
				},
				{
					ID: 2,
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Drifted,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: runExpected{
				Stdout: nljoin("s1", "s2"),
			},
		},
		{
			name: "stacks without id are ignored",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				"s:stack-without-id",
			},
			stacks: []cloud.StackResponse{
				{
					ID: 1,
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Failed,
				},
				{
					ID: 2,
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					Status: stack.Drifted,
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: runExpected{
				Stdout: nljoin("s1", "s2"),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			repository := tc.repository
			if repository == "" {
				repository = "github.com/terramate-io/terramate"
			}
			s.Git().SetRemoteURL("origin", repository)
			if len(tc.layout) > 0 {
				s.Git().CommitAll("all stacks committed")
			}
			for _, st := range tc.stacks {
				cloudtest.PutStack(t, testserver.DefaultOrgUUID, st)
			}
			cli := newCLI(t, filepath.Join(s.RootDir(), tc.workingDir))
			args := []string{"list"}
			args = append(args, tc.flags...)
			result := cli.run(args...)
			assertRunResult(t, result, tc.want)
		})
	}
}

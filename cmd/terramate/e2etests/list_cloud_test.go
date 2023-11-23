// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudListUnhealthy(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name       string
		layout     []string
		repository string
		stacks     []cloudstore.Stack
		flags      []string
		workingDir string
		want       runExpected
	}

	for _, tc := range []testcase{
		{
			name:       "only unhealthy filter is permitted",
			layout:     []string{"s:s1:id=s1"},
			repository: test.TempDir(t),
			flags:      []string{`--experimental-status=drifted`},
			want: runExpected{
				Status:      1,
				StderrRegex: "only unhealthy filter allowed",
			},
		},
		{
			name:       "local repository is not permitted with --experimental-status=unhealthy",
			layout:     []string{"s:s1:id=s1"},
			repository: test.TempDir(t),
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.OK,
						DeploymentStatus: deployment.OK,
						DriftStatus:      drift.OK,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "gitlab.com/unknown-io/other",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
				},
				{
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.OK,
						DeploymentStatus: deployment.OK,
						DriftStatus:      drift.OK,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
				},
				{
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Drifted,
						DeploymentStatus: deployment.OK,
						DriftStatus:      drift.Drifted,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
				},
				{
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Drifted,
						DeploymentStatus: deployment.OK,
						DriftStatus:      drift.Drifted,
					},
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
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.OK,
					},
				},
				{
					Stack: cloud.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Drifted,
						DeploymentStatus: deployment.OK,
						DriftStatus:      drift.Drifted,
					},
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
			t.Parallel()

			store, err := cloudstore.LoadDatastore(testserverJSONFile)
			assert.NoError(t, err)
			addr := startFakeTMCServer(t, store)

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

			org := store.MustOrgByName("terramate")
			for _, st := range tc.stacks {
				_, err := store.UpsertStack(org.UUID, st)
				assert.NoError(t, err)
			}
			env := removeEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr, "CI=")
			cli := newCLI(t, filepath.Join(s.RootDir(), tc.workingDir), env...)
			args := []string{"list"}
			args = append(args, tc.flags...)
			result := cli.run(args...)
			assertRunResult(t, result, tc.want)
		})
	}
}

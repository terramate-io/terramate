// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
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

func TestTriggerUnhealthyStacks(t *testing.T) {
	t.Parallel()

	const (
		stackID    = "my-stack-1"
		repository = "github.com/terramate-io/terramate"
	)

	store, err := cloudstore.LoadDatastore(testserverJSONFile)
	assert.NoError(t, err)

	addr := startFakeTMCServer(t, store)

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stacks/my-stack-1:id=` + stackID,
		`s:stacks/my-stack-2`,
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	org := store.MustOrgByName("terramate")

	_, err = store.UpsertStack(org.UUID, cloudstore.Stack{
		Stack: cloud.Stack{
			Repository: repository,
			MetaID:     stackID,
		},
		State: cloudstore.StackState{
			Status:           stack.Failed,
			DeploymentStatus: deployment.Failed,
			DriftStatus:      drift.OK,
		},
	})
	assert.NoError(t, err)

	git.SetRemoteURL("origin", fmt.Sprintf(`https://%s.git`, repository))
	env := removeEnv(os.Environ(), "CI")
	env = append(env, "TMC_API_URL=http://"+addr, "CI=")
	cli := newCLI(t, s.RootDir(), env...)
	assertRunResult(t, cli.run("experimental", "trigger", "--experimental-status=unhealthy"), runExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")

	assertRunResult(t, cli.listChangedStacks(),
		runExpected{
			Stdout: nljoin("stacks/my-stack-1"),
		},
	)
}

func TestCloudTriggerUnhealthy(t *testing.T) {
	t.Parallel()
	type want struct {
		trigger runExpected
		list    runExpected
	}
	type testcase struct {
		name       string
		layout     []string
		repository string
		stacks     []cloudstore.Stack
		flags      []string
		workingDir string
		want       want
	}

	for _, tc := range []testcase{
		{
			name:       "only unhealthy filter is permitted",
			layout:     []string{"s:s1:id=s1"},
			repository: test.TempDir(t),
			flags:      []string{`--experimental-status=drifted`},
			want: want{
				trigger: runExpected{
					Status:      1,
					StderrRegex: "only unhealthy filter allowed",
				},
			},
		},
		{
			name:       "local repository is not permitted with --experimental-status=unhealthy",
			layout:     []string{"s:s1:id=s1"},
			repository: test.TempDir(t),
			flags:      []string{`--experimental-status=unhealthy`},
			want: want{
				trigger: runExpected{
					Status:      1,
					StderrRegex: "unhealthy status filter does not work with filesystem based remotes",
				},
			},
		},
		{
			name: "no cloud stacks, no status flag, fail",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			want: want{
				trigger: runExpected{
					Status:      1,
					StderrRegex: "trigger command expects either a stack path or the --experimental-status flag",
				},
			},
		},
		{
			name: "no cloud stacks, asking for unhealthy, trigger nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			flags: []string{"--experimental-status=unhealthy"},
		},
		{
			name: "1 cloud stack healthy, other absent, trigger nothing",
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
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
		},
		{
			name: "1 cloud stack unhealthy but different repository, trigger nothing",
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
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
		},
		{
			name: "1 cloud stack unhealthy, other absent, trigger unhealthy",
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
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: want{
				trigger: runExpected{
					StdoutRegex: "Created trigger for stack",
				},
				list: runExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "1 cloud stack unhealthy, other ok, trigger unhealthy",
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
						DriftStatus:      drift.Unknown,
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
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--experimental-status=unhealthy`},
			want: want{
				trigger: runExpected{
					StdoutRegex: "Created trigger for stack",
				},
				list: runExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "no local stacks, 2 unhealthy stacks, trigger nothing",
			stacks: []cloudstore.Stack{
				{
					Stack: cloud.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Unknown,
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
			name: "2 local stacks, 2 same unhealthy stacks, trigger both",
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
						DriftStatus:      drift.Unknown,
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
			want: want{
				trigger: runExpected{
					StdoutRegex: "Created trigger for stack",
				},
				list: runExpected{
					Stdout: nljoin("s1", "s2"),
				},
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
						DriftStatus:      drift.Unknown,
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
			want: want{
				trigger: runExpected{
					StdoutRegex: "Created trigger for stack",
				},
				list: runExpected{
					Stdout: nljoin("s1", "s2"),
				},
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
			repositoryURL := tc.repository
			if repository == "" {
				repository = "github.com/terramate-io/terramate"
			}
			if !filepath.IsAbs(repository) {
				repositoryURL = fmt.Sprintf("https://%s.git", repository)
			}
			if len(tc.layout) > 0 {
				s.Git().CommitAll("all stacks committed")
			}
			s.Git().Push("main")
			s.Git().CheckoutNew("trigger-the-stacks")

			s.Git().SetRemoteURL("origin", repositoryURL)

			org := store.MustOrgByName("terramate")

			for _, st := range tc.stacks {
				_, err := store.UpsertStack(org.UUID, st)
				assert.NoError(t, err)
			}
			env := removeEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr, "CI=")
			cli := newCLI(t, filepath.Join(s.RootDir(), tc.workingDir), env...)
			args := []string{"experimental", "trigger"}
			args = append(args, tc.flags...)
			result := cli.run(args...)
			assertRunResult(t, result, tc.want.trigger)

			if tc.want.trigger.Status == 0 {
				s.Git().CommitAll("stacks triggered", true)
				assertRunResult(t, cli.listChangedStacks(), tc.want.list)
			}
		})
	}
}

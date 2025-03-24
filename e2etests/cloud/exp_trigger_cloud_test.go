// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestTriggerUnhealthyStacks(t *testing.T) {
	t.Parallel()

	const (
		stackID    = "my-stack-1"
		repository = "github.com/terramate-io/terramate"
	)

	store, defaultOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
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

	org := store.MustOrgByName(defaultOrg)

	_, err = store.UpsertStack(org.UUID, cloudstore.Stack{
		Stack: resources.Stack{
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
	env := RemoveEnv(os.Environ(), "CI")
	env = append(env, "TMC_API_URL=http://"+addr, "CI=")
	env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)
	cli := NewCLI(t, s.RootDir(), env...)
	AssertRunResult(t, cli.Run("trigger", "--status=unhealthy"), RunExpected{
		IgnoreStdout: true,
	})

	git.CommitAll("commit the trigger file")

	AssertRunResult(t, cli.ListChangedStacks(),
		RunExpected{
			Stdout: nljoin("stacks/my-stack-1"),
		},
	)
}

func TestCloudTriggerUnhealthy(t *testing.T) {
	t.Parallel()
	type want struct {
		trigger RunExpected
		list    RunExpected
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
			name:       "local repository is not permitted with --status=",
			layout:     []string{"s:s1:id=s1"},
			repository: test.TempDir(t),
			flags:      []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					Status:      1,
					StderrRegex: "status filters does not work with filesystem based remotes",
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
				trigger: RunExpected{
					Status:      1,
					StderrRegex: "trigger command expects either a stack path or the --status flag",
				},
			},
		},
		{
			name: "no cloud stacks, asking for unhealthy, trigger nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			flags: []string{"--status=unhealthy"},
		},
		{
			name: "1 cloud stack healthy, other absent, asking for unhealthy: trigger nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
		},
		{
			name: "1 cloud stack unhealthy but different repository, trigger nothing",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
		},
		{
			name: "1 cloud stack failed, other absent, asking for unhealthy, trigger the failed",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "deprecated --cloud-status alias",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
			flags: []string{`--cloud-status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "1 cloud stack failed, other ok, asking for unhealthy: trigger failed",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "1 cloud stack failed, other ok, asking for ok: trigger ok",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=ok`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s2"),
				},
			},
		},
		{
			name: "1 cloud stack failed, other ok, asking for healthy: trigger ok",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=ok`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s2"),
				},
			},
		},
		{
			name: "1 cloud stack failed, other ok, asking for failed: trigger failed",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=failed`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name: "1 cloud stack drifted, other ok, asking for drifted: trigger drifted",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
						MetaID:     "s1",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Drifted,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Drifted,
					},
				},
				{
					Stack: resources.Stack{
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
			flags: []string{`--status=drifted`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1"),
				},
			},
		},
		{
			name:   "no local stacks, 2 unhealthy stacks, trigger nothing",
			layout: []string{},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
		},
		{
			name: "2 local stacks, 2 same unhealthy stacks, trigger both",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
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
					Stack: resources.Stack{
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
					Stack: resources.Stack{
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
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("s1", "s2"),
				},
			},
		},
		{
			name:       "triggers stacks relative to workingDir",
			workingDir: "dir1/s1",
			layout: []string{
				"s:dir1/s1:id=s1",
				"s:dir1/s1/s1a:id=s1a",
				"s:dir2/s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
						MetaID:     "s1a",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Unknown,
					},
				},
				{
					Stack: resources.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("dir1/s1", "dir1/s1/s1a"),
				},
			},
		},
		{
			name:       "triggers stacks relative to workingDir inside leaf stack",
			workingDir: "dir2/s2",
			layout: []string{
				"s:dir1/s1:id=s1",
				"s:dir1/s1/s1a:id=s1a",
				"s:dir2/s2:id=s2",
			},
			stacks: []cloudstore.Stack{
				{
					Stack: resources.Stack{
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
					Stack: resources.Stack{
						MetaID:     "s1a",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Unknown,
					},
				},
				{
					Stack: resources.Stack{
						MetaID:     "s2",
						Repository: "github.com/terramate-io/terramate",
					},
					State: cloudstore.StackState{
						Status:           stack.Failed,
						DeploymentStatus: deployment.Failed,
						DriftStatus:      drift.Unknown,
					},
				},
			},
			flags: []string{`--status=unhealthy`},
			want: want{
				trigger: RunExpected{
					StdoutRegex: "Created change trigger for stack",
				},
				list: RunExpected{
					Stdout: nljoin("dir2/s2"),
				},
			},
		},
	} {
		tc := tc

		for _, argStr := range []string{"experimental trigger", "trigger"} {
			t.Run(tc.name+", "+argStr, func(t *testing.T) {
				t.Parallel()

				store, defaultOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
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

				org := store.MustOrgByName(defaultOrg)

				for _, st := range tc.stacks {
					_, err := store.UpsertStack(org.UUID, st)
					assert.NoError(t, err)
				}
				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, "TMC_API_URL=http://"+addr, "CI=")
				env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)
				cli := NewCLI(t, filepath.Join(s.RootDir(), tc.workingDir), env...)
				args := strings.Split(argStr, " ")
				args = append(args, tc.flags...)
				result := cli.Run(args...)
				AssertRunResult(t, result, tc.want.trigger)

				if tc.want.trigger.Status == 0 {
					s.Git().CommitAll("stacks triggered", true)
					cli = NewCLI(t, filepath.Join(s.RootDir()), env...)
					AssertRunResult(t, cli.ListChangedStacks(), tc.want.list)
				}
			})
		}
	}
}

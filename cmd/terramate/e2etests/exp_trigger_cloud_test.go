// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver"
	cloudtest "github.com/terramate-io/terramate/test/cloud"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestTriggerUnhealthyStacks(t *testing.T) {
	const (
		stackID    = "my-stack-1"
		repository = "github.com/terramate-io/terramate"
	)

	startFakeTMCServer(t)

	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stacks/my-stack-1:id=` + stackID,
		`s:stacks/my-stack-2`,
	})

	git := s.Git()
	git.CommitAll("all")
	git.Push("main")
	git.CheckoutNew("trigger-the-stack")

	cloudtest.PutStack(t, testserver.DefaultOrgUUID, cloud.StackResponse{
		ID: 1,
		Stack: cloud.Stack{
			Repository: repository,
			MetaID:     stackID,
		},
		Status: stack.Failed,
	})

	git.SetRemoteURL("origin", fmt.Sprintf(`https://%s.git`, repository))

	cli := newCLI(t, s.RootDir())
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
	type want struct {
		trigger runExpected
		list    runExpected
	}
	type testcase struct {
		name       string
		layout     []string
		repository string
		stacks     []cloud.StackResponse
		flags      []string
		workingDir string
		want       want
	}

	startFakeTMCServer(t)

	for _, tc := range []testcase{
		{
			name:       "only unhealthy filter is permitted",
			layout:     []string{"s:s1:id=s1"},
			repository: t.TempDir(),
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
			repository: t.TempDir(),
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
			name: "1 cloud stack unhealthy but different repository, trigger nothing",
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
			name: "1 cloud stack unhealthy, other absent, trigger unhealthy",
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
			name:   "no local stacks, 2 unhealthy stacks, trigger nothing",
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
			name: "2 local stacks, 2 same unhealthy stacks, trigger both",
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
			for _, st := range tc.stacks {
				cloudtest.PutStack(t, testserver.DefaultOrgUUID, st)
			}
			cli := newCLI(t, filepath.Join(s.RootDir(), tc.workingDir))
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

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncDriftStatus(t *testing.T) {
	type want struct {
		run    runExpected
		drifts cloud.DriftStackPayloadRequests
	}
	type testcase struct {
		name       string
		layout     []string
		runflags   []string
		workingDir string
		cmd        []string
		want       want
		runMode    runMode
	}

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1:id=s1",
				"s:s2", // missing id
			},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "flag requires that selected stacks contain an ID field",
				},
			},
		},
		{
			name:   "command not found -- set status=failed",
			layout: []string{"s:stack:id=stack"},
			cmd:    []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/stack",
							MetaName:   "stack",
							MetaID:     "stack",
						},
						Status: stack.Failed,
					},
				},
			},
		},
		{
			name: "failed cmd cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			cmd: []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Failed,
					},
				},
			},
		},

		{
			name: "both failed stacks and continueOnError",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Failed,
					},
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s2",
							MetaName:   "s2",
							MetaID:     "s2",
						},
						Status: stack.Failed,
					},
				},
			},
		},
		{
			name: "failed cmd and continueOnError",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				"f:s2/test.txt:test",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{testHelperBin, "cat", "test.txt"},
			want: want{
				run: runExpected{
					Status:      1,
					Stdout:      "test",
					StderrRegex: `(no such file or directory|The system cannot find the file specified)`,
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Failed,
					},
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s2",
							MetaName:   "s2",
							MetaID:     "s2",
						},
						Status: stack.OK,
					},
				},
			},
		},
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack:id=stack"},
			runMode: hangRun,
			want: want{
				run: runExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			},
		},
		{
			name:    "skipped subsequent stacks",
			layout:  []string{"s:s1:id=s1", "s:s2:id=s2"},
			runMode: sleepRun,
			want: want{
				run: runExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Failed,
					},
				},
			},
		},
		{
			name:   "basic drift sync",
			layout: []string{"s:stack:id=stack"},
			cmd: []string{
				testHelperBin, "exit", "2",
			},
			want: want{
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/stack",
							MetaName:   "stack",
							MetaID:     "stack",
						},
						Status: stack.Drifted,
					},
				},
			},
		},
		{
			name: "only stacks inside working dir are synced",
			layout: []string{
				"s:parent:id=parent",
				"s:parent/child:id=child",
			},
			workingDir: "parent/child",
			want: want{
				run: runExpected{
					Status: 0,
					Stdout: "/parent/child\n",
				},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/parent/child",
							MetaName:   "child",
							MetaID:     "child",
						},
						Status: stack.OK,
					},
				},
			},
		},
		{
			name: "multiple drifted stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			cmd: []string{testHelperBin, "exit", "2"},
			want: want{
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Drifted,
					},
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s2",
							MetaName:   "s2",
							MetaID:     "s2",
						},
						Status: stack.Drifted,
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// NOTE: this test needs to be serial :-(
			startFakeTMCServer(t)

			s := sandbox.New(t)

			s.BuildTree(tc.layout)
			s.Git().CommitAll("all stacks committed")
			cli := newCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)))
			runflags := []string{"--cloud-sync-drift-status"}
			runflags = append(runflags, tc.runflags...)

			fixture := cli.newRunFixture(tc.runMode, s.RootDir(), runflags...)
			fixture.cmd = tc.cmd // if empty, uses the runFixture default cmd.
			result := fixture.run()
			assertRunResult(t, result, tc.want.run)
			assertRunDrifts(t, tc.want.drifts)
		})
	}
}

func assertRunDrifts(t *testing.T, expectedDrifts cloud.DriftStackPayloadRequests) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := cloud.Request[cloud.DriftStackPayloadRequests](ctx, &cloud.Client{
		BaseURL:    "http://localhost:3001",
		Credential: &credential{},
	}, "GET", cloud.DriftsPath+"/"+testserver.DefaultOrgUUID, nil)
	assert.NoError(t, err)

	if diff := cmp.Diff(res, expectedDrifts); diff != "" {
		t.Logf("want: %+v", expectedDrifts)
		t.Logf("got: %+v", res)
		t.Fatal(diff)
	}
}

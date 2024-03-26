// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package cloud_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncDeploymentWithSignals(t *testing.T) {
	t.Parallel()
	type want struct {
		run    RunExpected
		events eventsResponse
	}
	type testcase struct {
		name       string
		layout     []string
		runflags   []string
		workingDir string
		cmd        []string
		want       want
		runMode    RunMode
	}

	for _, tc := range []testcase{
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack:id=stack"},
			runMode: HangRun,
			want: want{
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "canceled"},
				},
			},
		},
		{
			name:    "canceled subsequent stacks",
			layout:  []string{"s:s1:id=s1", "s:s1/s2:id=s1_s2"},
			runMode: SleepRun,
			want: want{
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"s1":    []string{"pending", "running", "failed"},
					"s1_s2": []string{"pending", "canceled"},
				},
			},
		},
	} {

		for _, isParallel := range []bool{false, true} {
			tc := tc
			isParallel := isParallel
			name := tc.name
			if isParallel {
				name += "-parallel"
			}

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				cloudData, err := cloudstore.LoadDatastore(testserverJSONFile)
				assert.NoError(t, err)
				addr := startFakeTMCServer(t, cloudData)

				s := sandbox.New(t)
				s.BuildTree(tc.layout)

				s.Git().CommitAll("all stacks committed")
				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, "TMC_API_URL=http://"+addr)

				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)

				s.Git().SetRemoteURL("origin", testRemoteRepoURL)

				runflags := []string{
					"--disable-safeguards=git-out-of-sync",
					"--cloud-sync-deployment",
				}
				if isParallel {
					runflags = append(runflags, "--parallel", "5")
					tc.want.run.IgnoreStdout = true
					tc.want.run.IgnoreStderr = true
				}
				runflags = append(runflags, tc.runflags...)

				fixture := cli.NewRunFixture(tc.runMode, s.RootDir(), runflags...)
				fixture.Command = tc.cmd // if empty, uses the runFixture default cmd.
				result := fixture.Run()
				AssertRunResult(t, result, tc.want.run)
				assertRunEvents(t, cloudData, s.Git().RevParse("HEAD"), tc.want.events)
			})
		}
	}
}

func TestCLIRunWithCloudSyncDriftStatusWithSignals(t *testing.T) {
	t.Parallel()
	type want struct {
		run    RunExpected
		drifts expectedDriftStackPayloadRequests
	}
	type testcase struct {
		name       string
		layout     []string
		runflags   []string
		workingDir string
		cmd        []string
		want       want
		runMode    RunMode
	}

	for _, tc := range []testcase{
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack:id=stack"},
			runMode: HangRun,
			want: want{
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			},
		},
		{
			name:    "skipped subsequent stacks",
			layout:  []string{"s:s1:id=s1", "s:s1/s2:id=s2"},
			runMode: SleepRun,
			want: want{
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/s1",
								MetaName:      "s1",
								MetaID:        "s1",
							},
							Status:   drift.Failed,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
	} {
		for _, isParallel := range []bool{false, true} {
			tc := tc
			isParallel := isParallel
			name := tc.name
			if isParallel {
				name += "-parallel"
			}
			t.Run(name, func(t *testing.T) {
				cloudData, err := cloudstore.LoadDatastore(testserverJSONFile)
				assert.NoError(t, err)
				addr := startFakeTMCServer(t, cloudData)

				s := sandbox.New(t)

				s.BuildTree(tc.layout)
				s.Git().CommitAll("all stacks committed")

				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, "TMC_API_URL=http://"+addr)

				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
				s.Git().SetRemoteURL("origin", testRemoteRepoURL)
				runflags := []string{
					"--disable-safeguards=git-out-of-sync",
					"--cloud-sync-drift-status",
				}
				if isParallel {
					runflags = append(runflags, "--parallel=5")
					tc.want.run.IgnoreStdout = true
					tc.want.run.IgnoreStderr = true
				}
				runflags = append(runflags, tc.runflags...)

				fixture := cli.NewRunFixture(tc.runMode, s.RootDir(), runflags...)
				fixture.Command = tc.cmd // if empty, uses the runFixture default cmd.
				minStartTime := time.Now().UTC()
				result := fixture.Run()
				maxEndTime := time.Now().UTC()
				AssertRunResult(t, result, tc.want.run)
				assertRunDrifts(t, cloudData, addr, tc.want.drifts, minStartTime, maxEndTime)
			})
		}
	}
}

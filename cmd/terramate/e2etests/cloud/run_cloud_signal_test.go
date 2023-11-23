// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package cloud_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
		skipIDGen  bool
		workingDir string
		cmd        []string
		want       want
		runMode    RunMode
	}

	for _, tc := range []testcase{
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack"},
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
			layout:  []string{"s:s1", "s:s2"},
			runMode: SleepRun,
			want: want{
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "canceled"},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cloudData, err := cloudstore.LoadDatastore(testserverJSONFile)
			assert.NoError(t, err)
			addr := startFakeTMCServer(t, cloudData)

			s := sandbox.New(t)
			var genIdsLayout []string
			ids := []string{}
			if !tc.skipIDGen {
				for _, layout := range tc.layout {
					if layout[0] == 's' {
						if strings.Contains(layout, "id=") {
							t.Fatalf("testcases should not contain stack IDs but found %s", layout)
						}
						id := strings.ToLower(strings.Replace(layout[2:]+"-id-"+t.Name(), "/", "-", -1))
						if len(id) > 64 {
							id = id[:64]
						}
						ids = append(ids, id)
						layout += ":id=" + id
					}
					genIdsLayout = append(genIdsLayout, layout)
				}
			} else {
				genIdsLayout = tc.layout
			}

			s.BuildTree(genIdsLayout)
			s.Git().CommitAll("all stacks committed")
			env := RemoveEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr)

			cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			uuid, err := uuid.NewRandom()
			assert.NoError(t, err)
			runid := uuid.String()
			cli.AppendEnv = []string{"TM_TEST_RUN_ID=" + runid}

			runflags := []string{"--cloud-sync-deployment"}
			runflags = append(runflags, tc.runflags...)

			fixture := cli.NewRunFixture(tc.runMode, s.RootDir(), runflags...)
			fixture.Command = tc.cmd // if empty, uses the runFixture default cmd.
			result := fixture.Run()
			AssertRunResult(t, result, tc.want.run)
			assertRunEvents(t, cloudData, runid, ids, tc.want.events)
		})
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
			layout:  []string{"s:s1:id=s1", "s:s2:id=s2"},
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
								Repository:    "local",
								DefaultBranch: "main",
								Path:          "/s1",
								MetaName:      "s1",
								MetaID:        "s1",
							},
							Status: drift.Failed,
						},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cloudData, err := cloudstore.LoadDatastore(testserverJSONFile)
			assert.NoError(t, err)
			addr := startFakeTMCServer(t, cloudData)

			s := sandbox.New(t)

			s.BuildTree(tc.layout)
			s.Git().CommitAll("all stacks committed")

			env := RemoveEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr)

			cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			runflags := []string{"--cloud-sync-drift-status"}
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

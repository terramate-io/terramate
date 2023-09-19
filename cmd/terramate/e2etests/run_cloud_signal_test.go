// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin

package e2etest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncDeploymentWithSignals(t *testing.T) {
	type want struct {
		run    runExpected
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
		runMode    runMode
	}

	startFakeTMCServer(t)

	for _, tc := range []testcase{
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack"},
			runMode: hangRun,
			want: want{
				run: runExpected{
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
			runMode: sleepRun,
			want: want{
				run: runExpected{
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
			cli := newCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)))
			uuid, err := uuid.NewRandom()
			assert.NoError(t, err)
			runid := uuid.String()
			cli.appendEnv = []string{"TM_TEST_RUN_ID=" + runid}

			runflags := []string{"--cloud-sync-deployment"}
			runflags = append(runflags, tc.runflags...)

			fixture := cli.newRunFixture(tc.runMode, s.RootDir(), runflags...)
			fixture.cmd = tc.cmd // if empty, uses the runFixture default cmd.
			result := fixture.run()
			assertRunResult(t, result, tc.want.run)
			assertRunEvents(t, runid, ids, tc.want.events)
		})
	}
}

func TestCLIRunWithCloudSyncDriftStatusWithSignals(t *testing.T) {
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

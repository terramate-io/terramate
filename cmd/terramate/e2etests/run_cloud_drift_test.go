// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncDriftStatus(t *testing.T) {
	type want struct {
		run    runExpected
		drifts cloud.DriftStackPayloadRequests
	}
	type testcase struct {
		name             string
		layout           []string
		runflags         []string
		workingDir       string
		cmd              []string
		driftDetailASCII string
		driftDetailJSON  string
		want             want
	}

	const testPlanASCII = "here goes the terraform plan output\n"

	testUnsanitizedPlanJSON, err := os.ReadFile("_testdata/unsanitized.plan.json")
	assert.NoError(t, err)
	testSanitizedPlanJSON, err := os.ReadFile("_testdata/sanitized.plan.json")
	assert.NoError(t, err)

	testSanitizedPlanJSON = bytes.TrimSpace(testSanitizedPlanJSON)

	absPlanFilePath := test.WriteFile(t, t.TempDir(), "out.tfplan", ``)

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1:id=s1",
				"s:s2", // missing id
			},
			cmd: []string{testHelperBin, "echo", "ok"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: string(clitest.ErrCloudStacksWithoutID),
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
			runflags:   []string{`--eval`},
			cmd:        []string{testHelperBinAsHCL, "echo", "${terramate.stack.path.absolute}"},
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
		{
			name: "using --cloud-sync-terraform-plan-file with non-existent plan file",
			layout: []string{
				"s:s1:id=s1",
			},
			runflags: []string{
				`--cloud-sync-terraform-plan-file=out.tfplan`,
			},
			cmd: []string{testHelperBin, "exit", "2"},
			want: want{
				run: runExpected{
					StderrRegexes: []string{
						string(clitest.ErrCloudTerraformPlanFile),
						"skipping",
					},
				},
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
				},
			},
		},
		{
			name: "using --cloud-sync-terraform-plan-file with absolute path",
			layout: []string{
				"s:s1:id=s1",
			},
			runflags: []string{
				fmt.Sprintf(`--cloud-sync-terraform-plan-file=%s`, absPlanFilePath),
			},
			cmd: []string{testHelperBin, "exit", "2"},
			want: want{
				run: runExpected{
					StderrRegexes: []string{
						string(clitest.ErrCloudInvalidTerraformPlanFilePath),
						"skipping",
					},
				},
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
				},
			},
		},
		{
			name: "using --cloud-sync-terraform-plan-file=out.tfplan",
			layout: []string{
				"s:s1:id=s1",
				`f:s1/out.tfplan:`,
				"s:s2:id=s2",
				`f:s2/out.tfplan:`,
			},
			runflags: []string{
				`--cloud-sync-terraform-plan-file=out.tfplan`,
			},
			cmd:              []string{testHelperBin, "exit", "2"},
			driftDetailASCII: testPlanASCII,
			driftDetailJSON:  string(testUnsanitizedPlanJSON),
			want: want{
				run: runExpected{},
				drifts: cloud.DriftStackPayloadRequests{
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s1",
							MetaName:   "s1",
							MetaID:     "s1",
						},
						Status: stack.Drifted,
						Details: &cloud.DriftDetails{
							Provisioner:    "terraform",
							ChangesetASCII: testPlanASCII,
							ChangesetJSON:  string(testSanitizedPlanJSON),
						},
					},
					{
						Stack: cloud.Stack{
							Repository: "local",
							Path:       "/s2",
							MetaName:   "s2",
							MetaID:     "s2",
						},
						Status: stack.Drifted,
						Details: &cloud.DriftDetails{
							Provisioner:    "terraform",
							ChangesetASCII: testPlanASCII,
							ChangesetJSON:  string(testSanitizedPlanJSON),
						},
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

			env := removeEnv(os.Environ(), "CI", "GITHUB_ACTIONS")
			env = append(env, `TM_TEST_TERRAFORM_SHOW_ASCII_OUTPUT=`+tc.driftDetailASCII)
			env = append(env, `TM_TEST_TERRAFORM_SHOW_JSON_OUTPUT=`+tc.driftDetailJSON)
			cli := newCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			cli.prependToPath(filepath.Dir(testHelperBin))
			runflags := []string{"run", "--cloud-sync-drift-status"}
			runflags = append(runflags, tc.runflags...)
			runflags = append(runflags, "--")
			runflags = append(runflags, tc.cmd...)
			result := cli.run(runflags...)
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

	// TODO(i4k): skip checking interpolated commands for now because of the hack
	// for making the --eval work with the helper binary in a portable way.
	// We can't portably predict the command because when using --eval the
	// whole argument list is interpolated, including the program name, and then
	// on Windows it requires a special escaped string.
	// See variable `testHelperBinAsHCL`.
	if diff := cmp.Diff(res, expectedDrifts, cmpopts.IgnoreFields(cloud.DriftStackPayloadRequest{}, "Command")); diff != "" {
		t.Logf("want: %+v", expectedDrifts)
		t.Logf("got: %+v", res)
		t.Fatal(diff)
	}
}

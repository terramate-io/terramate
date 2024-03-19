// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

type expectedDriftStackPayloadRequests []expectedDriftStackPayloadRequest
type expectedDriftStackPayloadRequest struct {
	cloud.DriftStackPayloadRequest

	ChangesetASCIIRegexes []string
}

var expectedMetadata *cloud.DeploymentMetadata

func init() {
	expectedMetadata = &cloud.DeploymentMetadata{
		GitMetadata: cloud.GitMetadata{
			GitCommitAuthorName:  "terramate tests",
			GitCommitAuthorEmail: "terramate@mineiros.io",
			GitCommitTitle:       "all stacks committed",
		},
	}
}

func TestCLIRunWithCloudSyncDriftStatus(t *testing.T) {
	t.Parallel()
	type want struct {
		run    RunExpected
		drifts expectedDriftStackPayloadRequests
	}
	type testcase struct {
		name          string
		layout        []string
		runflags      []string
		env           []string
		workingDir    string
		defaultBranch string
		cmd           []string
		want          want
	}

	absPlanFilePath := test.WriteFile(t, test.TempDir(t), "out.tfplan", ``)

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1:id=s1",
				"s:s2", // missing id
			},
			cmd: []string{HelperPath, "echo", "ok"},
			want: want{
				run: RunExpected{
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
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/stack",
								MetaName:      "stack",
								MetaID:        "stack",
							},
							Status:   drift.Failed,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name: "failed cmd cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
			},
			cmd: []string{"non-existent-command"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
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
		{
			name: "both failed stacks and continueOnError",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{"non-existent-command"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
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
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/s1/s2",
								MetaName:      "s2",
								MetaID:        "s2",
							},
							Status:   drift.Failed,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name: "one failed cmd and continueOnError",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
				"f:s1/s2/test.txt:test",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{HelperPath, "cat", "test.txt"},
			want: want{
				run: RunExpected{
					Status:      1,
					Stdout:      "test",
					StderrRegex: `(no such file or directory|The system cannot find the file specified)`,
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
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/s1/s2",
								MetaName:      "s2",
								MetaID:        "s2",
							},
							Status:   drift.OK,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name:   "basic drift sync",
			layout: []string{"s:stack:id=stack"},
			cmd: []string{
				HelperPath, "exit", "2",
			},
			want: want{
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/stack",
								MetaName:      "stack",
								MetaID:        "stack",
							},
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name:   "basic drift sync with uppercase stack id",
			layout: []string{"s:stack:id=STACK"},
			cmd: []string{
				HelperPath, "exit", "2",
			},
			want: want{
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/stack",
								MetaName:      "stack",
								MetaID:        "stack",
							},
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
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
			cmd:        []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			want: want{
				run: RunExpected{
					Status: 0,
					Stdout: "/parent/child\n",
				},
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/parent/child",
								MetaName:      "child",
								MetaID:        "child",
							},
							Status:   drift.OK,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name: "multiple drifted stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
			},
			cmd: []string{HelperPath, "exit", "2"},
			want: want{
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
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
					},
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/s1/s2",
								MetaName:      "s2",
								MetaID:        "s2",
							},
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
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
			cmd: []string{HelperPath, "exit", "2"},
			want: want{
				run: RunExpected{
					StderrRegexes: []string{
						clitest.CloudSkippingTerraformPlanSync,
					},
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
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
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
			cmd: []string{HelperPath, "exit", "2"},
			want: want{
				run: RunExpected{
					StderrRegexes: []string{
						string(clitest.ErrCloudInvalidTerraformPlanFilePath),
						"skipping",
					},
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
							Status:   drift.Drifted,
							Metadata: expectedMetadata,
						},
					},
				},
			},
		},
		{
			name: "using --cloud-sync-terraform-plan-file=out.tfplan",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
				"copy:s1:testdata/cloud-sync-drift-plan-file",
				"copy:s1/s2:testdata/cloud-sync-drift-plan-file",
				"run:s1:terraform init",
				"run:s1/s2:terraform init",
			},
			runflags: []string{
				`--cloud-sync-terraform-plan-file=out.tfplan`,
			},
			env: []string{
				`TF_VAR_content=my secret`,
			},
			cmd: []string{
				"terraform", "plan", "-no-color", "-detailed-exitcode", "-out=out.tfplan",
			},
			want: want{
				run: RunExpected{
					StdoutRegexes: []string{
						`Terraform used the selected providers to generate the following execution`,
						`local_file.foo will be created`,
					},
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
							Status: drift.Drifted,
							Details: &cloud.ChangesetDetails{
								Provisioner:   "terraform",
								ChangesetJSON: loadJSONPlan(t, "testdata/cloud-sync-drift-plan-file/sanitized.plan.json"),
							},
							Metadata: expectedMetadata,
						},
						ChangesetASCIIRegexes: []string{
							`Terraform used the selected providers to generate the following execution`,
							`local_file.foo will be created`,
						},
					},
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "main",
								Path:          "/s1/s2",
								MetaName:      "s2",
								MetaID:        "s2",
							},
							Status: drift.Drifted,
							Details: &cloud.ChangesetDetails{
								Provisioner:   "terraform",
								ChangesetJSON: loadJSONPlan(t, "testdata/cloud-sync-drift-plan-file/sanitized.plan.json"),
							},
							Metadata: expectedMetadata,
						},
						ChangesetASCIIRegexes: []string{
							`Terraform used the selected providers to generate the following execution`,
							`local_file.foo will be created`,
						},
					},
				},
			},
		},
		{
			name: "drift with different default branch",
			layout: []string{
				"s:stack:id=stack",
				`f:cfg.tm.hcl:terramate {
					config {
						git {
							default_branch = "trunk"
						}
					}
				}`,
			},
			cmd: []string{
				HelperPath, "exit", "2",
			},
			defaultBranch: "trunk",
			want: want{
				drifts: expectedDriftStackPayloadRequests{
					{
						DriftStackPayloadRequest: cloud.DriftStackPayloadRequest{
							Stack: cloud.Stack{
								Repository:    normalizedTestRemoteRepo,
								DefaultBranch: "trunk",
								Path:          "/stack",
								MetaName:      "stack",
								MetaID:        "stack",
							},
							Status:   drift.Drifted,
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
				t.Parallel()

				cloudData, err := cloudstore.LoadDatastore(testserverJSONFile)
				assert.NoError(t, err)
				addr := startFakeTMCServer(t, cloudData)

				defaultBranch := tc.defaultBranch
				if defaultBranch == "" {
					defaultBranch = "main"
				}

				s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
					LocalBranchName:         defaultBranch,
					DefaultRemoteName:       "origin",
					DefaultRemoteBranchName: defaultBranch,
				})

				s.Env, _ = test.PrependToPath(os.Environ(), filepath.Dir(TerraformTestPath))

				s.BuildTree(tc.layout)
				s.Git().CommitAll("all stacks committed")

				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, tc.env...)
				env = append(env, "TMC_API_URL=http://"+addr)
				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
				cli.PrependToPath(filepath.Dir(TerraformTestPath))
				s.Git().SetRemoteURL("origin", testRemoteRepoURL)
				runflags := []string{
					"run",
					"--disable-safeguards=git-out-of-sync",
					"--quiet",
					"--cloud-sync-drift-status",
				}
				if isParallel {
					runflags = append(runflags, "-j", "5")
					tc.want.run.IgnoreStdout = true
					tc.want.run.IgnoreStderr = true
				}
				runflags = append(runflags, tc.runflags...)
				runflags = append(runflags, "--")
				runflags = append(runflags, tc.cmd...)

				minStartTime := time.Now().UTC()
				result := cli.Run(runflags...)
				maxEndTime := time.Now().UTC()
				AssertRunResult(t, result, tc.want.run)
				assertRunDrifts(t, cloudData, addr, tc.want.drifts, minStartTime, maxEndTime)

				for _, wantDrift := range tc.want.drifts {
					cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(wantDrift.Stack.Path[1:])), env...)
					res := cli.Run("cloud", "drift", "show")

					if wantDrift.Status != drift.Drifted {
						AssertRunResult(t, res, RunExpected{
							Status:      0,
							StdoutRegex: "is not drifted",
						})
					} else {
						if wantDrift.Details == nil || (wantDrift.Details.ChangesetASCII == "" && wantDrift.Details.ChangesetJSON == "") {
							AssertRunResult(t, res, RunExpected{
								Status:      1,
								StderrRegex: "is drifted, but no details are available.",
							})
						} else {
							AssertRunResult(t, res, RunExpected{
								Status: 0,
								StdoutRegexes: []string{
									"Terraform used the selected providers to generate the following execution",
									`local_file.foo will be created`,
								},
							})
						}
					}
				}
			})
		}
	}
}

func assertRunDrifts(t *testing.T, cloudData *cloudstore.Data, tmcAddr string, expectedDrifts expectedDriftStackPayloadRequests, minStartTime, maxEndTime time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := &cloud.Client{
		BaseURL:    "http://" + tmcAddr,
		Credential: &credential{},
	}
	res, err := cloud.Request[cloud.DriftStackPayloadRequests](ctx, client, "GET", client.URL(path.Join(cloud.DriftsPath, string(cloudData.MustOrgByName("terramate").UUID))), nil)
	assert.NoError(t, err)

	if len(expectedDrifts) != len(res) {
		t.Fatalf("expected %d drifts but found %d: %+v", len(expectedDrifts), len(res), res)
	}

	for i, expected := range expectedDrifts {
		got := res[i]
		if diff := cmp.Diff(got, expected.DriftStackPayloadRequest,
			// Ignore hard to predict fields
			// They are validated (for existence) in the testserver anyway.
			cmpopts.IgnoreFields(cloud.GitMetadata{}, "GitCommitSHA", "GitCommitAuthorTime"),

			// TODO(i4k): skip checking interpolated commands for now because of the hack
			// for making the --eval work with the helper binary in a portable way.
			// We can't portably predict the command because when using --eval the
			// whole argument list is interpolated, including the program name, and then
			// on Windows it requires a special escaped string.
			// See variable `HelperPathAsHCL`.
			cmpopts.IgnoreFields(cloud.DriftStackPayloadRequest{}, "Command", "Details", "StartedAt", "FinishedAt")); diff != "" {
			t.Logf("want: %+v", expectedDrifts)
			t.Logf("got: %+v", got)
			t.Fatal(diff)
		}

		if (expected.DriftStackPayloadRequest.Details == nil) !=
			(got.Details == nil) {
			t.Fatalf("drift_detals is absent in expected or got result: want %v != got %v",
				expected.DriftStackPayloadRequest.Details,
				got.Details,
			)
		}

		assertDriftRunDuration(t, &got, minStartTime, maxEndTime)

		if expected.DriftStackPayloadRequest.Details == nil {
			continue
		}

		assert.EqualStrings(t, expected.DriftStackPayloadRequest.Details.Provisioner,
			got.Details.Provisioner,
			"provisioner mismatch",
		)

		if len(expected.ChangesetASCIIRegexes) > 0 {
			changeSetASCII := got.Details.ChangesetASCII

			for _, changesetASCIIRegex := range expected.ChangesetASCIIRegexes {
				matched, err := regexp.MatchString(changesetASCIIRegex, changeSetASCII)
				assert.NoError(t, err, "failed to compile regex %q", changesetASCIIRegex)

				if !matched {
					t.Errorf("changeset_ascii=\"%s\" does not match regex %q",
						changeSetASCII,
						changesetASCIIRegex,
					)
				}
			}

		} else {
			assert.EqualStrings(t, expected.DriftStackPayloadRequest.Details.ChangesetASCII,
				got.Details.ChangesetASCII,
				"changeset_ascii mismatch")
		}

		if got.Details.ChangesetJSON == expected.Details.ChangesetJSON {
			continue
		}

		var gotPlan, wantPlan tfjson.Plan

		assert.NoError(t, json.Unmarshal([]byte(got.Details.ChangesetJSON), &gotPlan))
		assert.NoError(t, json.Unmarshal([]byte(expected.Details.ChangesetJSON), &wantPlan))

		if diff := cmp.Diff(gotPlan, wantPlan, cmpopts.IgnoreFields(tfjson.Plan{}, "Timestamp", "FormatVersion")); diff != "" {
			t.Logf("want: %+v", expected.Details.ChangesetJSON)
			t.Logf("got: %+v", got.Details.ChangesetJSON)
			t.Fatal(diff)
		}
	}
}

func assertDriftRunDuration(t *testing.T, got *cloud.DriftStackPayloadRequest, minStartTime, maxEndTime time.Time) {
	hasStartTime := got.StartedAt != nil
	hasEndTime := got.FinishedAt != nil
	assert.IsTrue(t, hasStartTime == hasEndTime, "hasStartTime(%s) == hasEndTime(%s)", hasStartTime, hasEndTime)

	if got.Status == drift.OK || got.Status == drift.Drifted || got.Status == drift.Unknown {
		assert.IsTrue(t, hasStartTime, "hasStartTime for status %s", got.Status)
		assert.IsTrue(t, hasEndTime, "hasEndTime for status %s", got.Status)
	}

	if got.StartedAt != nil && got.FinishedAt != nil {
		assert.IsTrue(t, minStartTime.Compare(*got.StartedAt) <= 0, "StartedAt(%s) >= %s", *got.StartedAt, minStartTime)
		assert.IsTrue(t, maxEndTime.Compare(*got.FinishedAt) >= 0, "FinishedAt(%s) <= %s", *got.FinishedAt, maxEndTime)

		assert.IsTrue(t, got.StartedAt.Compare(*got.FinishedAt) <= 0, "StartedAt(%s) <= FinishedAt(%s)", *got.StartedAt, *got.FinishedAt)
	}
}

func loadJSONPlan(t *testing.T, fname string) string {
	fname = filepath.FromSlash(fname)
	jsonBytes := test.ReadFile(t, filepath.Dir(fname), filepath.Base(fname))
	var plan tfjson.Plan
	assert.NoError(t, json.Unmarshal(jsonBytes, &plan))
	plan.TerraformVersion = TerraformVersion
	jsonNewBytes, err := json.Marshal(&plan)
	assert.NoError(t, err)
	return string(jsonNewBytes)
}

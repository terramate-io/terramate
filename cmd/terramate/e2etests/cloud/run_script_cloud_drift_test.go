// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptRunDriftStatus(t *testing.T) {
	type want struct {
		run    RunExpected
		drifts expectedDriftStackPayloadRequests
	}
	type testcase struct {
		name          string
		layout        []string
		defaultBranch string
		workingDir    string
		env           []string
		cmd           []string
		want          want
	}

	absPlanFilePath := test.WriteFile(t, test.TempDir(t), "out.tfplan", ``)
	absPlanFilePathAsHCL := fmt.Sprintf(`${tm_chomp(<<-EOF
		%s
	EOF
	)}`, absPlanFilePath)

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1:id=s1",
				"s:s2", // missing id
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", `["echo", "ok", {
							cloud_sync_drift_status = true
						}]`),
					),
				).String(),
			},
			cmd: []string{"cmd"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: string(clitest.ErrCloudStacksWithoutID),
				},
			},
		},
		{
			name: "command not found -- set status=failed",
			layout: []string{
				"s:stack:id=stack",
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", `["non-existent-command", {
							cloud_sync_drift_status = true
						}]`),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", `["non-existent-command", {
							cloud_sync_drift_status = true
						}]`),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
			name: "basic drift sync",
			layout: []string{
				"s:stack:id=stack",
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "exit", "2", {
							cloud_sync_drift_status = true
						}]`, HelperPathAsHCL)),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "echo", "${terramate.stack.path.absolute}", {
							cloud_sync_drift_status = true
						}]`, HelperPathAsHCL)),
					),
				).String(),
			},
			workingDir: "parent/child",
			cmd:        []string{"cmd"},
			want: want{
				run: RunExpected{
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "exit", "2", {
							cloud_sync_drift_status = true
						}]`, HelperPathAsHCL)),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "exit", "2", {
							cloud_sync_drift_status = true
							cloud_sync_terraform_plan_file = "out.tfplan"
						}]`, HelperPathAsHCL)),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "exit", "2", {
							cloud_sync_drift_status = true
							cloud_sync_terraform_plan_file = "%s"
						}]`, HelperPathAsHCL, absPlanFilePathAsHCL)),
					),
				).String(),
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command",
							`["terraform", "plan", "-no-color", "-detailed-exitcode", "-out=out.tfplan", {
							cloud_sync_drift_status = true
							cloud_sync_terraform_plan_file = "out.tfplan"
						}]`),
					),
				).String(),
			},
			env: []string{
				`TF_VAR_content=my secret`,
			},
			cmd: []string{"cmd"},
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
				"f:script.tm:" + Block("script",
					Labels("cmd"),
					Str("description", "test"),
					Block("job",
						Expr("command", fmt.Sprintf(
							`["%s", "exit", "2", {
							cloud_sync_drift_status = true
						}]`, HelperPathAsHCL)),
					),
				).String(),
			},
			cmd:           []string{"cmd"},
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
		tc := tc
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

				tc.layout = append(tc.layout, "f:terramate.tm:"+Block("terramate",
					Block("config",
						Expr("experiments", `["scripts"]`))).String())

				s.BuildTree(tc.layout)
				s.Git().CommitAll("all stacks committed")

				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, tc.env...)
				env = append(env, "TMC_API_URL=http://"+addr)
				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
				cli.PrependToPath(filepath.Dir(TerraformTestPath))
				s.Git().SetRemoteURL("origin", normalizedTestRemoteRepo)
				runflags := []string{
					"script",
					"run",
					"--disable-safeguards=git-out-of-sync",
					"--quiet",
				}
				if isParallel {
					runflags = append(runflags, "--parallel", "5")
					tc.want.run.IgnoreStdout = true
					tc.want.run.IgnoreStderr = true
				}
				runflags = append(runflags, "--")
				runflags = append(runflags, tc.cmd...)

				minStartTime := time.Now().UTC()
				result := cli.Run(runflags...)
				maxEndTime := time.Now().UTC()
				AssertRunResult(t, result, tc.want.run)
				assertRunDrifts(t, cloudData, addr, tc.want.drifts, minStartTime, maxEndTime)
			})
		}
	}
}

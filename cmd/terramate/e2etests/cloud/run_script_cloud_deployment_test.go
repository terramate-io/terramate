// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIScriptRunWithCloudSyncDeployment(t *testing.T) {
	t.Parallel()

	makeScriptDef := func(name string, syncDeployment bool, plan string) string {
		return Block("script",
			Labels(name),
			Str("description", "no"),
			Block("job",
				Expr("command", fmt.Sprintf(`["echo", "${terramate.stack.name}", {
			cloud_sync_deployment = %v,
			cloud_sync_terraform_plan_file = "%s"
		}]`, syncDeployment, plan)),
			),
		).String()
	}

	type want struct {
		run    RunExpected
		events eventsResponse
	}
	type testcase struct {
		name       string
		layout     []string
		scripts    map[string]string
		workingDir string
		scriptCmd  string
		want       want
	}

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			scripts: map[string]string{
				"deploy.tm": makeScriptDef("deploy", true, ""),
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: string(clitest.ErrCloudStacksWithoutID),
				},
			},
		},
		{
			name: "failed script command",
			layout: []string{
				"s:stack:id=stack",
				`f:stack/scripts.tm:script deploy {
					description = "no"
					job {
						command = ["echooooo", "${terramate.stack.name}", {
							cloud_sync_deployment = true
						}]
					}
				}`,
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "failed"},
				},
			},
		},
		{
			name: "failed script cmd cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s1_s2",
				`f:scripts.tm:script deploy {
					description = "no"
					job {
						command = ["echooooo", "${terramate.stack.name}", {
							cloud_sync_deployment = true
						}]
					}
				}`,
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"s1":    []string{"pending", "running", "failed"},
					"s1_s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name:   "basic success",
			layout: []string{"s:stack:id=stack"},
			scripts: map[string]string{
				"deploy.tm": makeScriptDef("deploy", true, ""),
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Stdout: "stack\n",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:   "basic success - uppercase ID",
			layout: []string{"s:stack:id=STACK"},
			scripts: map[string]string{
				"deploy.tm": makeScriptDef("deploy", true, ""),
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Stdout: "stack\n",
				},
				events: eventsResponse{
					// CLI lower the case of stack ID when syncing to the cloud.
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "multiple stacks - sync all",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				"s:s3:id=s3",
			},
			scripts: map[string]string{
				"deploy.tm": makeScriptDef("deploy", true, ""),
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Stdout: "s1\ns2\ns3\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
					"s3": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "multiple stacks - partial sync",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				"s:s3:id=s3",
			},
			scripts: map[string]string{
				"deploy_sync.tm":       makeScriptDef("deploy", true, ""),
				"s2/deploy_no_sync.tm": makeScriptDef("deploy", false, ""),
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Stdout: "s1\ns2\ns3\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s3": []string{"pending", "running", "ok"},
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

				layout := tc.layout
				for path, def := range tc.scripts {
					layout = append(layout, fmt.Sprintf("f:%s:%s", path, def))
				}

				layout = append(layout, `f:terramate.tm:
					terramate {
					config {
						experiments = ["scripts"]
					}
					}`)

				s.BuildTree(layout)
				s.Git().CommitAll("all stacks committed")

				env := RemoveEnv(os.Environ(), "CI")
				env = append(env, "TMC_API_URL=http://"+addr)
				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)

				s.Git().SetRemoteURL("origin", testRemoteRepoURL)

				scriptArgs := []string{"--quiet", "--disable-safeguards=git-out-of-sync"}
				if isParallel {
					scriptArgs = append(scriptArgs, "--parallel=5")
					// For the parallel test, we ignore output validation, since the print order is non-deterministic.
					tc.want.run.IgnoreStderr = true
					tc.want.run.IgnoreStdout = true
				}
				scriptArgs = append(scriptArgs, tc.scriptCmd)

				result := cli.RunScript(scriptArgs...)
				AssertRunResult(t, result, tc.want.run)
				assertRunEvents(t, cloudData, s.Git().RevParse("HEAD"), tc.want.events)
			})
		}
	}
}

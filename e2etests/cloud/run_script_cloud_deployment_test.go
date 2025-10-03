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
	"github.com/terramate-io/terramate/ui/tui/clitest"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
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
				Expr("command", fmt.Sprintf(`["helper", "echo", "${terramate.stack.name}", {
			sync_deployment = %v,
			terraform_plan_file = "%s"
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
							sync_deployment = true
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
					"stack": []string{"pending", "failed"},
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
							sync_deployment = true
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
					"s1":    []string{"pending", "failed"},
					"s1_s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name: "script command not found without sync still cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s1_s2",
				`f:scripts.tm:script deploy {
					description = "no"
					job {
						commands = [
						  ["echooooo", "${terramate.stack.name}"],
						  ["helper", "echo", "ok", {sync_deployment = true}]
						]
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
					"s1":    []string{"pending", "failed"},
					"s1_s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name: "script command failing without sync still sync status=failed and cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s1_s2",
				`f:scripts.tm:script deploy {
					description = "no"
					job {
						commands = [
						  ["helper", "exit", "1"],
						  ["helper", "echo", "ok", {sync_deployment = true}]
						]
					}
				}`,
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "execution failed",
				},
				events: eventsResponse{
					"s1":    []string{"pending", "failed"},
					"s1_s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name: "script command without failing sync failed with previous successful commands, still sync status=failed and cancels execution of subsequent stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s1_s2",
				`f:scripts.tm:script deploy {
					description = "no"
					job {
						commands = [
						  ["helper", "echo", "ok"],
						  ["helper", "exit", "1"],
						  ["echo", "ok", {sync_deployment = true}]
						]
					}
				}`,
			},
			scriptCmd: "deploy",
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "execution failed",
					Stdout:      nljoin("ok"),
				},
				events: eventsResponse{
					"s1":    []string{"pending", "failed"},
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

				cloudData, defaultOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
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

				env := RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS")
				env = append(env, "TMC_API_URL=http://"+addr)
				env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)
				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
				cli.PrependToPath(filepath.Dir(HelperPath))

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

func TestScriptRunIOBuffering(t *testing.T) {
	t.Parallel()

	cloudData, defaultOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
	assert.NoError(t, err)
	addr := startFakeTMCServer(t, cloudData)

	s := sandbox.NewWithGitConfig(t, sandbox.GitConfig{
		LocalBranchName:         "main",
		DefaultRemoteName:       "origin",
		DefaultRemoteBranchName: "main",
	})
	s.Env = os.Environ()

	s.BuildTree([]string{
		"s:s1:id=s1",
		"f:terramate.tm:" + Block("terramate",
			Block("config",
				Expr("experiments", `["scripts"]`),
			),
		).String(),
		"f:script.tm:" + Block("script",
			Labels("cmd"),
			Str("description", "test"),

			Block("job",
				Expr("command", fmt.Sprintf(
					`["%s", "prompt", {
						sync_deployment = true
					}]`, HelperPathAsHCL)),
			),
		).String(),
	})

	s.Git().CommitAll("all stacks committed")

	env := RemoveEnv(s.Env, "CI", "GITHUB_ACTIONS")
	env = append(env, "TMC_API_URL=http://"+addr)
	env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)
	cli := NewCLI(t, s.RootDir(), env...)
	s.Git().SetRemoteURL("origin", testRemoteRepoURL)

	runArgs := []string{"script", "run", "--disable-safeguards=git-out-of-sync", "--quiet", "cmd"}

	// TODO(snk): Testing with one job unbuffered, one buffered should work, but is non-deterministic.
	// This is not a valid use case, but would be good to understand eventually where the non-deterministic aspect comes from.

	ioFunc := func(stdin InteractiveWrite, expectStdout, _ ExpectedRead) {
		expectStdout("are you sure?\n")
		stdin("my input\n")
		expectStdout("prompt: \nyou entered: my input\n")
	}

	result := cli.RunInteractive(ioFunc, runArgs...)

	AssertRunResult(t, result, RunExpected{})
}

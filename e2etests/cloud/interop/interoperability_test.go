// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build interop

package interop_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
)

const defaultTarget = "interop-tests"

func TestInteropCloudSyncPreview(t *testing.T) {
	for _, stackpath := range []string{
		"testdata/interop-stacks/basic-drift",
		"testdata/interop-stacks/basic-drift-uppercase-id",
	} {
		t.Run("preview: "+path.Base(stackpath), func(t *testing.T) {
			env := os.Environ()

			eventFile := os.Getenv("GITHUB_EVENT_PATH")
			if eventFile != "" {
				// check if exported GITHUB_EVENT_FILE is of type `pull_request`
				var obj map[string]interface{}
				content, err := os.ReadFile(eventFile)
				assert.NoError(t, err)
				assert.NoError(t, json.Unmarshal(content, &obj))
				if obj["pull_request"] == nil {
					eventFile = ""
				}
			}
			if eventFile == "" {
				eventFile = datapath(t, "testdata/event_pull_request.json")
				env = append(env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", eventFile))
			}
			t.Logf("using GITHUB_EVENT_FILE=%s", eventFile)
			env = append(env, "GITHUB_ACTIONS=1")
			tmcli := NewInteropCLI(t, datapath(t, stackpath), env...)
			AssertRunResult(t,
				tmcli.Run("run", "--quiet",
					"--",
					TerraformTestPath,
					"init",
				),
				RunExpected{
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)
			AssertRunResult(t,
				tmcli.Run("run", "--quiet",
					"--sync-preview",
					"--terraform-plan-file=out.plan",
					"--target", defaultTarget,
					"--",
					TerraformTestPath,
					"plan",
					"-out=out.plan",
					"--detailed-exitcode",
				),
				RunExpected{
					StderrRegexes: []string{
						"Preview created",
					},
					IgnoreStdout: true,
				},
			)
		})
	}
}

func TestInteropSyncDeployment(t *testing.T) {
	for _, stackpath := range []string{
		"testdata/interop-stacks/empty",
		"testdata/interop-stacks/empty-uppercase-id",
	} {
		t.Run("deployment: "+path.Base(stackpath), func(t *testing.T) {
			tmcli := NewInteropCLI(t, datapath(t, stackpath))
			AssertRunResult(t, tmcli.Run("list"), RunExpected{
				Stdout: nljoin("."),
			})
			AssertRunResult(t,
				tmcli.Run(
					"run",
					"--quiet",
					"--sync-deployment",
					"--target", defaultTarget,
					"--",
					HelperPath, "false",
				),
				RunExpected{
					IgnoreStderr: true,
					Status:       1,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=unhealthy", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=failed", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// fix the failed stacks
			AssertRun(t, tmcli.Run(
				"run", "--quiet", "--status=failed", "--sync-deployment", "--target", defaultTarget, "--", HelperPath, "true",
			))

			AssertRunResult(t,
				tmcli.Run("list", "--status=ok", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRun(t, tmcli.Run("list", "--status=unhealthy", "--target", defaultTarget))
			AssertRun(t, tmcli.Run("list", "--status=failed", "--target", defaultTarget))
			AssertRun(t, tmcli.Run("list", "--status=drifted", "--target", defaultTarget))
		})
	}
}

func TestInteropDrift(t *testing.T) {
	for _, stackpath := range []string{
		"testdata/interop-stacks/basic-drift",
		"testdata/interop-stacks/basic-drift-uppercase-id",
	} {
		t.Run("drift: "+filepath.Base(stackpath), func(t *testing.T) {
			tmcli := NewInteropCLI(t, datapath(t, stackpath))
			AssertRunResult(t, tmcli.Run("list"), RunExpected{
				Stdout: nljoin("."),
			})
			// initialize the providers
			AssertRunResult(t,
				tmcli.Run("run", "--quiet", "--", TerraformTestPath, "init"),
				RunExpected{
					Status:       0,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)

			// basic drift, without details
			AssertRunResult(t,
				tmcli.Run("run", "--quiet", "--sync-drift-status", "--target", defaultTarget, "--", TerraformTestPath, "plan", "-detailed-exitcode"),
				RunExpected{
					Status:       0,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=unhealthy", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=drifted", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// Check if there are no drift details
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show", "--target", defaultTarget), RunExpected{
					StderrRegex: "Stack .*? is drifted, but no details are available",
					Status:      1,
				},
			)

			// complete drift
			AssertRunResult(t,
				tmcli.Run(
					"run", "--sync-drift-status", "--target", defaultTarget, "--terraform-plan-file=out.plan", "--",
					TerraformTestPath, "plan", "-out=out.plan", "-detailed-exitcode",
				),
				RunExpected{
					Status:       0,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=unhealthy", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--status=drifted", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// Check the drift details
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show", "--target", defaultTarget), RunExpected{
					StdoutRegexes: []string{
						"hello world", // content of the file
						"local_file",  // name of the resource
					},
					Status: 0,
				},
			)

			// check reseting the drift status to OK
			AssertRun(t, tmcli.Run("run", "--quiet", "--status=drifted", "--sync-drift-status", "--target", defaultTarget, "--", HelperPath, "exit", "0"))
			AssertRun(t, tmcli.Run("list", "--status=unhealthy", "--target", defaultTarget))
			AssertRun(t, tmcli.Run("list", "--status=drifted", "--target", defaultTarget))
			AssertRunResult(t,
				tmcli.Run("list", "--status=ok", "--target", defaultTarget), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show", "--target", defaultTarget),
				RunExpected{
					StdoutRegex: "Stack .*? is not drifted",
					Status:      0,
				},
			)
		})
	}
}

func datapath(t *testing.T, path string) string {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	return filepath.Join(wd, filepath.FromSlash(path))
}

func nljoin(stacks ...string) string {
	return strings.Join(stacks, "\n") + "\n"
}

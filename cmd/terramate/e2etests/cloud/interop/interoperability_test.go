// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build interop

package interop_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
)

func TestInteropCloudSyncPreview(t *testing.T) {
	for _, stackpath := range []string{
		"testdata/interop-stacks/basic-drift",
		"testdata/interop-stacks/basic-drift-uppercase-id",
	} {
		t.Run("preview: "+path.Base(stackpath), func(t *testing.T) {
			env := os.Environ()
			env = append(env, fmt.Sprintf("GITHUB_EVENT_PATH=%s", datapath(t, "testdata/event_pull_request.json")))
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
					"--cloud-sync-preview",
					"--cloud-sync-terraform-plan-file=out.plan",
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
				tmcli.Run("run", "--quiet", "--cloud-sync-deployment", "--", HelperPath, "false"),
				RunExpected{
					IgnoreStderr: true,
					Status:       1,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=unhealthy"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=failed"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// fix the failed stacks
			AssertRun(t, tmcli.Run("run", "--quiet", "--cloud-status=failed", "--cloud-sync-deployment", "--", HelperPath, "true"))

			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=ok"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRun(t, tmcli.Run("list", "--cloud-status=unhealthy"))
			AssertRun(t, tmcli.Run("list", "--cloud-status=failed"))
			AssertRun(t, tmcli.Run("list", "--cloud-status=drifted"))
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
				tmcli.Run("run", "--quiet", "--cloud-sync-drift-status", "--", TerraformTestPath, "plan", "-detailed-exitcode"),
				RunExpected{
					Status:       0,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=unhealthy"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=drifted"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// Check if there are no drift details
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show"), RunExpected{
					StderrRegex: "Stack .*? is drifted, but no details are available",
					Status:      1,
				},
			)

			// complete drift
			AssertRunResult(t,
				tmcli.Run(
					"run", "--cloud-sync-drift-status", "--cloud-sync-terraform-plan-file=out.plan", "--",
					TerraformTestPath, "plan", "-out=out.plan", "-detailed-exitcode",
				),
				RunExpected{
					Status:       0,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=unhealthy"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=drifted"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			// Check the drift details
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show"), RunExpected{
					StdoutRegexes: []string{
						"hello world", // content of the file
						"local_file",  // name of the resource
					},
					Status: 0,
				},
			)

			// check reseting the drift status to OK
			AssertRun(t, tmcli.Run("run", "--quiet", "--cloud-status=drifted", "--cloud-sync-drift-status", "--", HelperPath, "exit", "0"))
			AssertRun(t, tmcli.Run("list", "--cloud-status=unhealthy"))
			AssertRun(t, tmcli.Run("list", "--cloud-status=drifted"))
			AssertRunResult(t,
				tmcli.Run("list", "--cloud-status=ok"), RunExpected{
					Stdout: nljoin("."),
				},
			)
			AssertRunResult(t,
				tmcli.Run("cloud", "drift", "show"),
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

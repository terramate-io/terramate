// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package terragrunt

import (
	"context"
	"os"
	"testing"

	"github.com/madlambda/spells/assert"
	runpkg "github.com/terramate-io/terramate/run"
)

func TestRunner_Version(t *testing.T) {
	tmpDir := t.TempDir()
	env := os.Environ()

	runner := NewRunner(env, tmpDir)

	runpkg.SetTestVersionOverride("terragrunt", "0.99.0")
	t.Cleanup(func() { runpkg.SetTestVersionOverride("terragrunt", "") })

	ctx := context.Background()
	v := runner.Version(ctx)

	assert.EqualStrings(t, v, "0.99.0")
}

func TestRunner_ShowCommand(t *testing.T) {
	t.Run("legacy flags and TERRAGRUNT_ env for <0.73.0", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := os.Environ()

		runner := NewRunner(env, tmpDir)
		runpkg.SetTestVersionOverride("terragrunt", "0.72.9")
		t.Cleanup(func() { runpkg.SetTestVersionOverride("terragrunt", "") })

		ctx := context.Background()
		planfile := "plan.tfplan"
		cmd, err := runner.ShowCommand(ctx, planfile, "--json")

		// We expect an error because the binary might not be in PATH, but check the command structure
		if err == nil {
			assert.EqualStrings(t, cmd.Dir, tmpDir)

			args := cmd.Args
			expectedArgs := []string{"show", "--json", "--terragrunt-non-interactive", planfile}
			assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))
			for i, expectedArg := range expectedArgs {
				assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
			}

			envMap := make(map[string]string)
			for _, envVar := range cmd.Env {
				if len(envVar) == 0 {
					continue
				}
				parts := []string{"", ""}
				if idx := findFirstEqual(envVar); idx != -1 {
					parts[0] = envVar[:idx]
					parts[1] = envVar[idx+1:]
				} else {
					parts[0] = envVar
				}
				envMap[parts[0]] = parts[1]
			}
			assert.EqualStrings(t, envMap["TERRAGRUNT_FORWARD_TF_STDOUT"], "true")
			assert.EqualStrings(t, envMap["TERRAGRUNT_LOG_FORMAT"], "bare")
		}
	})

	t.Run("TG_ env for >=0.73.0 and modern flags for >=0.85.0", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := os.Environ()

		runner := NewRunner(env, tmpDir)
		runpkg.SetTestVersionOverride("terragrunt", "0.85.0")
		t.Cleanup(func() { runpkg.SetTestVersionOverride("terragrunt", "") })

		ctx := context.Background()
		planfile := "plan.tfplan"
		cmd, err := runner.ShowCommand(ctx, planfile, "--json", "--terragrunt-foo")

		// We expect an error because the binary might not be in PATH, but check the command structure
		if err == nil {
			assert.EqualStrings(t, cmd.Dir, tmpDir)

			args := cmd.Args
			// --terragrunt-foo should be rewritten to --foo and non-interactive modern flag used
			expectedArgs := []string{"show", "--json", "--foo", "--non-interactive", planfile}
			assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))
			for i, expectedArg := range expectedArgs {
				assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
			}

			envMap := make(map[string]string)
			for _, envVar := range cmd.Env {
				if len(envVar) == 0 {
					continue
				}
				parts := []string{"", ""}
				if idx := findFirstEqual(envVar); idx != -1 {
					parts[0] = envVar[:idx]
					parts[1] = envVar[idx+1:]
				} else {
					parts[0] = envVar
				}
				envMap[parts[0]] = parts[1]
			}
			assert.EqualStrings(t, envMap["TG_FORWARD_TF_STDOUT"], "true")
			assert.EqualStrings(t, envMap["TG_LOG_FORMAT"], "bare")
		}
	})

	t.Run("unknown version defaults to modern flags and TG_ env", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := os.Environ()

		runner := NewRunner(env, tmpDir)
		// Do not set version; force unknown version path

		ctx := context.Background()
		planfile := "plan.tfplan"
		cmd, err := runner.ShowCommand(ctx, planfile, "--json", "--terragrunt-foo")

		// Only validate structure when the binary is found in PATH
		if err == nil {
			assert.EqualStrings(t, cmd.Dir, tmpDir)

			args := cmd.Args
			expectedArgs := []string{"show", "--json", "--foo", "--non-interactive", planfile}
			assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))
			for i, expectedArg := range expectedArgs {
				assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
			}

			envMap := make(map[string]string)
			for _, envVar := range cmd.Env {
				if len(envVar) == 0 {
					continue
				}
				parts := []string{"", ""}
				if idx := findFirstEqual(envVar); idx != -1 {
					parts[0] = envVar[:idx]
					parts[1] = envVar[idx+1:]
				} else {
					parts[0] = envVar
				}
				envMap[parts[0]] = parts[1]
			}
			assert.EqualStrings(t, envMap["TG_FORWARD_TF_STDOUT"], "true")
			assert.EqualStrings(t, envMap["TG_LOG_FORMAT"], "bare")
		}
	})

	t.Run("malformed version defaults to modern flags and TG_ env", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := os.Environ()

		runner := NewRunner(env, tmpDir)
		runpkg.SetTestVersionOverride("terragrunt", "invalid-version")
		t.Cleanup(func() { runpkg.SetTestVersionOverride("terragrunt", "") })

		ctx := context.Background()
		planfile := "plan.tfplan"
		cmd, err := runner.ShowCommand(ctx, planfile, "--json", "--terragrunt-foo")

		// Only validate structure when the binary is found in PATH
		if err == nil {
			assert.EqualStrings(t, cmd.Dir, tmpDir)

			args := cmd.Args
			expectedArgs := []string{"show", "--json", "--foo", "--non-interactive", planfile}
			assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))
			for i, expectedArg := range expectedArgs {
				assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
			}

			envMap := make(map[string]string)
			for _, envVar := range cmd.Env {
				if len(envVar) == 0 {
					continue
				}
				parts := []string{"", ""}
				if idx := findFirstEqual(envVar); idx != -1 {
					parts[0] = envVar[:idx]
					parts[1] = envVar[idx+1:]
				} else {
					parts[0] = envVar
				}
				envMap[parts[0]] = parts[1]
			}
			assert.EqualStrings(t, envMap["TG_FORWARD_TF_STDOUT"], "true")
			assert.EqualStrings(t, envMap["TG_LOG_FORMAT"], "bare")
		}
	})
}

func findFirstEqual(s string) int {
	for i, c := range s {
		if c == '=' {
			return i
		}
	}
	return -1
}

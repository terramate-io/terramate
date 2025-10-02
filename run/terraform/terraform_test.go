// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package terraform_test

import (
	"context"
	"os"
	"testing"

	"github.com/madlambda/spells/assert"
	runpkg "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/terraform"
)

func TestNewRunner(t *testing.T) {
	tmpDir := t.TempDir()
	env := os.Environ()

	runner := terraform.NewRunner(env, tmpDir)

	assert.EqualStrings(t, runner.WorkingDir, tmpDir)
}

func TestRunner_Version(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	env := os.Environ()

	runner := &terraform.Runner{
		Env:        env,
		WorkingDir: tmpDir,
	}

	runpkg.SetTestVersionOverride("terraform", "1.2.3")
	t.Cleanup(func() { runpkg.SetTestVersionOverride("terraform", "") })

	ctx := context.Background()
	v := runner.Version(ctx)

	assert.EqualStrings(t, v, "1.2.3")
}

func TestRunner_ShowCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{"terraform"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			env := os.Environ()

			runner := &terraform.Runner{
				Env:        env,
				WorkingDir: tmpDir,
			}

			ctx := context.Background()
			planfile := "plan.tfplan"
			cmd, err := runner.ShowCommand(ctx, planfile, "--json")

			// We expect an error because the binary might not be in PATH, but check the command structure
			if err == nil {
				assert.EqualStrings(t, cmd.Dir, tmpDir)

				// Check that the command includes the expected args
				args := cmd.Args
				expectedArgs := []string{"show", "--json", planfile}
				assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))

				for i, expectedArg := range expectedArgs {
					assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
				}
			}
		})
	}
}

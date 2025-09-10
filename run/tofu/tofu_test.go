// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tofu_test

import (
	"context"
	"os"
	"testing"

	"github.com/madlambda/spells/assert"
	runpkg "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/tofu"
)

func TestNewRunner(t *testing.T) {
	tmpDir := t.TempDir()
	env := os.Environ()

	runner := tofu.NewRunner(env, tmpDir)

	assert.EqualStrings(t, runner.WorkingDir, tmpDir)
}

func TestRunner_Version(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	env := os.Environ()

	runner := &tofu.Runner{
		Env:        env,
		WorkingDir: tmpDir,
	}

	runpkg.SetTestVersionOverride("tofu", "9.9.9")
	t.Cleanup(func() { runpkg.SetTestVersionOverride("tofu", "") })

	ctx := context.Background()
	v := runner.Version(ctx)

	assert.EqualStrings(t, v, "9.9.9")
}

func TestRunner_ShowCommand(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	env := os.Environ()

	runner := &tofu.Runner{
		Env:        env,
		WorkingDir: tmpDir,
	}

	ctx := context.Background()
	planfile := "plan.tfplan"
	cmd, err := runner.ShowCommand(ctx, planfile, "--json")

	if err == nil {
		assert.EqualStrings(t, cmd.Dir, tmpDir)

		args := cmd.Args
		expectedArgs := []string{"show", "--json", planfile}
		assert.IsTrue(t, len(args) >= len(expectedArgs)+1, "expected at least %d args, got %d", len(expectedArgs)+1, len(args))

		for i, expectedArg := range expectedArgs {
			assert.EqualStrings(t, args[i+1], expectedArg, "arg %d mismatch", i+1)
		}
	}
}

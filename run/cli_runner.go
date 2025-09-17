// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"context"
	"os/exec"
)

// CLIRunner abstracts common CLI operations used by terraform, terragrunt and tofu runners.
// Implementations should be thin wrappers around the actual binaries and must not mutate
// provided arguments. Methods that execute commands return a prepared *exec.Cmd ready to run.
type CLIRunner interface {
	// Name returns the CLI runner name (e.g. "terraform", "terragrunt", "tofu").
	Name() string
	// Version returns the cached semantic version string of the underlying CLI.
	// Must use the shared version cache to avoid repeated shell-outs.
	Version(ctx context.Context) string
	ShowCommand(ctx context.Context, planfile string, flags ...string) (*exec.Cmd, error)
}

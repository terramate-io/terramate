// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package tofu provides utilities for executing OpenTofu commands with configurable arguments.
package tofu

import (
	"context"
	"os/exec"
	"sync"

	runpkg "github.com/terramate-io/terramate/run"
)

// Runner provides methods to execute OpenTofu commands with configurable arguments.
type Runner struct {
	// Environment variables for the command
	Env []string
	// Working directory for the command
	WorkingDir string

	// resolvedPath caches the absolute path to the tofu binary
	resolvedPath string
	resolveOnce  sync.Once
	resolveErr   error
}

// Ensure Runner implements run.CLIRunner at compile time.
var _ runpkg.CLIRunner = (*Runner)(nil)

// NewRunner creates a new OpenTofu runner with the given environment and working directory.
func NewRunner(env []string, workingDir string) *Runner {
	return &Runner{
		Env:        env,
		WorkingDir: workingDir,
	}
}

// Name returns the CLI name for this runner.
func (r *Runner) Name() string { return "tofu" }

// Version returns the semantic version string of the tofu binary.
func (r *Runner) Version(ctx context.Context) string {
	return r.tofuVersion(ctx)
}

func (r *Runner) command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	r.resolveOnce.Do(func() {
		r.resolvedPath, r.resolveErr = runpkg.LookPath("tofu", r.Env)
	})
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}

	cmd := exec.CommandContext(ctx, r.resolvedPath, args...)
	cmd.Dir = r.WorkingDir
	cmd.Env = r.Env

	return cmd, nil
}

// ShowCommand builds a command to execute `tofu show` for the given planfile.
func (r *Runner) ShowCommand(ctx context.Context, planfile string, flags ...string) (*exec.Cmd, error) {
	args := []string{"show"}
	args = append(args, flags...)
	args = append(args, planfile)

	return r.command(ctx, args...)
}

// tofuVersion returns the parsed semantic version (e.g., 1.6.2) of the resolved tofu binary.
// It shells out at most once per resolved binary path across the entire process.
func (r *Runner) tofuVersion(ctx context.Context) string {
	return runpkg.ResolveVersionFor(ctx, r.Env, "tofu", r.resolvedPath)
}

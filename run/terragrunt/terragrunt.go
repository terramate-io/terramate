// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package terragrunt provides utilities for executing terragrunt commands with configurable arguments.
package terragrunt

import (
	"context"
	"os/exec"
	"strings"
	"sync"

	runpkg "github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/versions"
)

// Runner provides methods to execute terragrunt commands with configurable arguments.
type Runner struct {
	// Environment variables for the command
	Env []string
	// Working directory for the command
	WorkingDir string

	// resolvedPath caches the absolute path to the terragrunt binary
	resolvedPath string
	resolveOnce  sync.Once
	resolveErr   error
}

// Ensure Runner implements run.CLIRunner at compile time.
var _ runpkg.CLIRunner = (*Runner)(nil)

// NewRunner creates a new terragrunt runner with the given environment and working directory.
func NewRunner(env []string, workingDir string) *Runner {
	return &Runner{
		Env:        env,
		WorkingDir: workingDir,
	}
}

// Name returns the CLI name for this runner.
func (r *Runner) Name() string { return "terragrunt" }

// Version returns the semantic version string of the terragrunt binary.
func (r *Runner) Version(ctx context.Context) string {
	return r.terragruntVersion(ctx)
}

func (r *Runner) command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	r.resolveOnce.Do(func() {
		r.resolvedPath, r.resolveErr = runpkg.LookPath("terragrunt", r.Env)
	})
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}

	cmd := exec.CommandContext(ctx, r.resolvedPath, args...)
	cmd.Dir = r.WorkingDir
	cmd.Env = r.Env

	return cmd, nil
}

// ShowCommand builds a command to execute `terragrunt show` for the given planfile.
func (r *Runner) ShowCommand(ctx context.Context, planfile string, flags ...string) (*exec.Cmd, error) {
	useModern := r.useModernFlags(ctx)
	useTG := r.useTGEnv(ctx)

	args := []string{"show"}
	if useModern {
		args = append(args, r.rewriteTerragruntFlags(flags)...)
		args = append(args, "--non-interactive")
	} else {
		args = append(args, flags...)
		args = append(args, "--terragrunt-non-interactive")
	}
	args = append(args, planfile)

	cmd, err := r.command(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Set terragrunt-specific environment variables
	env := make([]string, len(r.Env))
	copy(env, r.Env)
	if useTG {
		env = append(env, "TG_FORWARD_TF_STDOUT=true")
		env = append(env, "TG_LOG_FORMAT=bare")
	} else {
		env = append(env, "TERRAGRUNT_FORWARD_TF_STDOUT=true")
		env = append(env, "TERRAGRUNT_LOG_FORMAT=bare")
	}
	cmd.Env = env

	return cmd, nil
}

func (r *Runner) rewriteTerragruntFlags(flags []string) []string {
	if len(flags) == 0 {
		return flags
	}
	rewritten := make([]string, 0, len(flags))
	for _, f := range flags {
		if strings.HasPrefix(f, "--terragrunt-") {
			rewritten = append(rewritten, "--"+strings.TrimPrefix(f, "--terragrunt-"))
			continue
		}
		rewritten = append(rewritten, f)
	}
	return rewritten
}

func (r *Runner) useModernFlags(ctx context.Context) bool {
	ver := r.terragruntVersion(ctx)
	if ver == "" {
		// Default to modern behavior when version is unknown
		return true
	}
	ok, err := versions.Match(ver, ">= 0.85.0", false)
	if err != nil {
		// Default to modern behavior when version parsing fails
		return true
	}
	return ok
}

func (r *Runner) useTGEnv(ctx context.Context) bool {
	ver := r.terragruntVersion(ctx)
	if ver == "" {
		// Default to TG_* env when version is unknown
		return true
	}
	ok, err := versions.Match(ver, ">= 0.73.0", false)
	if err != nil {
		// Default to TG_* env when version parsing fails
		return true
	}
	return ok
}

func (r *Runner) terragruntVersion(ctx context.Context) string {
	return runpkg.ResolveVersionFor(ctx, r.Env, "terragrunt", r.resolvedPath)
}

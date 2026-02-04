// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package commands

import (
	"context"
	"io"

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// Command is an interface for a command handler.
type Command interface {
	// Name of the comamnd.
	Name() string

	// Requirements is a generic interface to query a command for common requirements
	// that will be fulfilled by the CLI before executing it.
	//
	// The result can either by a single requirement, or a RequirementsList.
	Requirements(context.Context, CLI) any

	// Exec executes the command.
	Exec(context.Context, CLI) error
}

// CLI is the interface for common CLI data required by commands.
type CLI interface {
	Version() string
	Product() string
	PrettyProduct() string

	WorkingDir() string

	Printers() printer.Printers
	Stdout() io.Writer
	Stderr() io.Writer
	Stdin() io.Reader

	// Config returns the userconfig.
	Config() cliconfig.Config

	// Engine returns the engine.
	// Will only be available for commands that have the engine requirement.
	Engine() *engine.Engine

	// Reload reloads the engine config and re-runs post-init hooks.
	// Must only be called by commands that have the engine requirement.
	Reload(ctx context.Context) error
}

// RequirementsList allows to return multiple requirements from Command.Requirements().
type RequirementsList []any

// HasRequirement checks if the given command has requirement of type T, and returns it if found.
// If the command returns a single requirement, it will be checked against T.
// If it returns a RequirementsList, each element will be checked.
func HasRequirement[T any](ctx context.Context, cli CLI, cmd Command) (*T, bool) {
	r := cmd.Requirements(ctx, cli)
	switch r := r.(type) {
	case RequirementsList:
		for _, req := range r {
			match, ok := req.(*T)
			if ok {
				return match, true
			}
		}
	case *T:
		return r, true
	}
	return nil, false
}

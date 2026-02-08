// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package extension defines the in-process extension interfaces.
package extension

import (
	"context"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/hcl"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

// CLI is the subset of TUI CLI used by extensions.
type CLI interface {
	Engine() *engine.Engine
	SetCommandAnalytics(cmd string, opts ...tel.MessageOpt)
}

// CommandSelector handles command selection for extensions.
type CommandSelector func(ctx context.Context, c CLI, command string, flags any) (commands.Command, error)

// BindingsSetupHandler configures DI bindings after config setup.
type BindingsSetupHandler func(c CLI, bindings *di.Bindings) error

// PostInitEngineHook runs after engine initialization.
type PostInitEngineHook func(ctx context.Context, c CLI) error

// Extension describes an in-process plugin that extends Terramate CLI.
type Extension interface {
	// Name returns the plugin name.
	Name() string
	// Version returns the plugin version.
	Version() string

	// HCLOptions returns HCL parser options to register custom blocks.
	HCLOptions() []hcl.Option

	// FlagSpec returns the custom flag spec (kong struct), or nil.
	FlagSpec() any

	// CommandSelector routes plugin commands.
	CommandSelector() CommandSelector

	// AfterConfigSetup returns DI binding setup handlers.
	AfterConfigSetup() []BindingsSetupHandler

	// PostInitEngineHooks returns hooks to run after engine init.
	PostInitEngineHooks() []PostInitEngineHook
}

// CLIInfo provides optional CLI metadata overrides for extensions.
type CLIInfo interface {
	Product() (product, prettyProduct string)
	BinaryName() string
	Description() string
}

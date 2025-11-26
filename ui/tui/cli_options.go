// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/hcl"
	"github.com/willabides/kongplete"
)

// WithVersion is an option to modify the CLI version.
// Default is `terramate.Version()`.
func WithVersion(v string) Option {
	return func(c *CLI) error {
		c.version = v
		return nil
	}
}

// WithStdout is an option to modify the CLI stdout channel.
func WithStdout(w io.Writer) Option {
	return func(c *CLI) error {
		c.state.stdout = w
		return nil
	}
}

// WithStderr is an option to modify the CLI stderr channel.
func WithStderr(w io.Writer) Option {
	return func(c *CLI) error {
		c.state.stderr = w
		return nil
	}
}

// WithStdin is an option to modify the CLI stdin channel.
func WithStdin(r io.Reader) Option {
	return func(c *CLI) error {
		c.state.stdin = r
		return nil
	}
}

// WithProduct is an option modify the product name.
// Default is `terramate`.
func WithProduct(product, prettyProduct string) Option {
	return func(c *CLI) error {
		c.product = product
		c.prettyProduct = prettyProduct
		return nil
	}
}

// WithBinaryName is an option modify the name of the binary.
// Default is `terramate`.
func WithBinaryName(name string) Option {
	return func(c *CLI) error {
		c.kongOpts.name = name
		return nil
	}
}

// WithDescription is an option to modify the project description.
func WithDescription(desc string) Option {
	return func(c *CLI) error {
		c.kongOpts.description = desc
		return nil
	}
}

// WithCompactHelp is an option to modify the helper compact option.
func WithCompactHelp(b bool) Option {
	return func(c *CLI) error {
		c.kongOpts.compactHelp = b
		return nil
	}
}

// WithExpandSubcommandsInHelp is an option to expand the subcommands in the help.
// By default they are not shown.
func WithExpandSubcommandsInHelp() Option {
	return func(c *CLI) error {
		c.kongOpts.NoExpandSubcommandsInHelp = false
		return nil
	}
}

// WithHelpPrinter allows for customizing the help output.
func WithHelpPrinter(p kong.HelpPrinter) Option {
	return func(c *CLI) error {
		c.kongOpts.helpPrinter = p
		return nil
	}
}

// RootFlagHandlers is a function signature for root flag handlers.
type RootFlagHandlers func(parsed any, cli *CLI) (name string, val any, run func(c *CLI, value any) error, isset bool)

// WithSpecHandler is an option to set the flag spec and handler for the CLI.
func WithSpecHandler(a any, commandSelector CommandSelector, checkers ...RootFlagHandlers) Option {
	return func(c *CLI) error {
		kongOptions := []kong.Option{
			kong.Name(c.kongOpts.name),
			kong.Description(c.kongOpts.description),
			kong.UsageOnError(),
			kong.ConfigureHelp(kong.HelpOptions{
				Compact:             c.kongOpts.compactHelp,
				NoExpandSubcommands: c.kongOpts.NoExpandSubcommandsInHelp,
			}),
			kong.Exit(func(status int) {
				// Avoid kong aborting entire process since we designed CLI as a lib
				c.kongExit = true
				c.kongExitStatus = status
			}),
			kong.Writers(c.state.stdout, c.state.stderr),
		}
		parser, err := kong.New(a, kongOptions...)

		c.parser = parser
		c.input = a
		c.commandSelector = commandSelector
		c.rootFlagCheckers = checkers

		if err != nil {
			return err
		}

		kongplete.Complete(c.parser,
			kongplete.WithPredictor("file", complete.PredictFiles("*")),
		)

		return nil
	}
}

// WithHCLOptions is an option to set the HCL options for the CLI.
func WithHCLOptions(hclOpts ...hcl.Option) Option {
	return func(c *CLI) error {
		c.hclOptions = hclOpts
		return nil
	}
}

// BindingsSetupHandler is the function signature for setup handlers.
type BindingsSetupHandler func(c *CLI, bindings *di.Bindings) error

// WithBeforeConfigSetup is an option to setup handlers that run before the config is loaded.
// Bindings set here will available in beforeConfigHandlers, but c.Engine() is not available there yet.
func WithBeforeConfigSetup(handlers ...BindingsSetupHandler) Option {
	return func(c *CLI) error {
		c.beforeConfigSetupHandlers = handlers
		return nil
	}
}

// WithAfterConfigSetup is an option to setup handlers that run after the config is loaded.
// Bindings set here will available in postInitEngineHooks and during command execution.
func WithAfterConfigSetup(handlers ...BindingsSetupHandler) Option {
	return func(c *CLI) error {
		c.afterConfigSetupHandlers = handlers
		return nil
	}
}

// WithPostInitEngineHooks is an option to run functions after engine initialization, but before commands.
func WithPostInitEngineHooks(hooks ...PostInitEngineHook) Option {
	return func(c *CLI) error {
		c.postInitEngineHooks = hooks
		return nil
	}
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
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
func WithProduct(product string) Option {
	return func(c *CLI) error {
		c.product = product
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
func WithSpecHandler(a any, beforeHandler, afterHandler Handler, checkers ...RootFlagHandlers) Option {
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
		c.rootFlagCheckers = checkers
		c.beforeConfigHandler = beforeHandler
		c.afterConfigHandler = afterHandler

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

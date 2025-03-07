package tui

import (
	"io"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/terramate-io/terramate/errors"
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

// WithName is an option modify the project name.
func WithName(name string) Option {
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

type rootFlagCheckers[T any] func(parsed T, cli *CLI) bool

// WithSpecHandler is an option to set the flag spec and handler for the CLI.
func WithSpecHandler[T any](a *T, beforeHandler, afterHandler Handler, checkers ...rootFlagCheckers[*T]) Option {
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

		var ignoreErr bool
		if err != nil && c.kongExit {
			for _, chk := range checkers {
				if chk(a, c) {
					ignoreErr = true
					break
				}
			}
		}

		if err != nil && !ignoreErr {
			return errors.E(err, "parsing CLI spec")
		}
		kongplete.Complete(parser,
			kongplete.WithPredictor("file", complete.PredictFiles("*")),
		)
		c.parser = parser
		c.input = a
		c.beforeConfigHandler = beforeHandler
		c.afterConfigHandler = afterHandler
		return nil
	}
}

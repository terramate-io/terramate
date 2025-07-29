// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"os"
	"time"

	_ "embed"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
	"github.com/terramate-io/terramate/ui/tui/out"

	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

// CLI is the Terramate command-line interface opaque type.
// The default flag spec is [input.Spec] and handler is [DefaultAfterConfigHandler].
type CLI struct {
	product string
	version string

	clicfg cliconfig.Config
	state  state

	printers printer.Printers

	// flag handling

	kongOpts       kongOptions
	kongExit       bool
	kongExitStatus int

	// The CLI engine works with any spec.
	input            any
	parser           *kong.Kong
	rootFlagCheckers []RootFlagHandlers
	hclOptions       []hcl.Option

	checkpointResponse chan *checkpoint.CheckResponse

	beforeConfigHandler Handler
	afterConfigHandler  Handler
}

// Handler is a function that handles the CLI configuration.
type Handler func(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error)

type kongOptions struct {
	name                      string
	description               string
	compactHelp               bool
	NoExpandSubcommandsInHelp bool
	helpPrinter               kong.HelpPrinter
}

type state struct {
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader

	engine *engine.Engine

	verbose int
	output  out.O
	wd      string
	uimode  engine.UIMode

	changeDetectionEnabled bool
}

// Option is a function that modifies the CLI behavior.
type Option func(*CLI) error

const (
	name = "terramate"
)

//go:embed cli_help.txt
var helpSummaryText string

const (
	defaultLogLevel = "warn"
	defaultLogFmt   = "console"
	defaultLogDest  = "stderr"
)

const terramateUserConfigDir = ".terramate.d"

// NewCLI creates a new CLI instance. The opts options modify the default CLI behavior.
func NewCLI(opts ...Option) (*CLI, error) {
	c := &CLI{
		product: name,
		version: terramate.Version(),
		kongOpts: kongOptions{
			name:                      name,
			description:               helpSummaryText,
			compactHelp:               true,
			NoExpandSubcommandsInHelp: true,
			helpPrinter:               terramateHelpPrinter,
		},
		state: state{
			stdout: os.Stdout,
			stderr: os.Stderr,
			stdin:  os.Stdin,
		},
	}
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}
	if c.parser == nil {
		err := WithSpecHandler(
			&FlagSpec{},
			DefaultBeforeConfigHandler,
			DefaultAfterConfigHandler,
			DefaultRootFlagHandlers()...)(c)

		if err != nil {
			return nil, err
		}
	}
	c.printers.Stdout = printer.NewPrinter(c.state.stdout)
	c.printers.Stderr = printer.NewPrinter(c.state.stderr)
	c.state.uimode = engine.HumanMode
	return c, nil
}

type contextStr string

// KongContext is the context key for the Kong context.
const KongContext contextStr = "kong.context"

// KongError is the context key for the Kong error.
const KongError contextStr = "kong.error"

// DidKongExit returns true if Kong intends to exit.
func (c *CLI) DidKongExit() bool {
	return c.kongExit
}

// InputSpec returns the CLI flags spec.
func (c *CLI) InputSpec() any { return c.input }

// Version returns the CLI Terramate version.
func (c *CLI) Version() string { return c.version }

// WorkingDir returns the CLI working directory.
func (c *CLI) WorkingDir() string { return c.state.wd }

// Config returns the CLI Terramate configuration.
func (c *CLI) Config() *config.Root { return c.state.engine.Config() }

// Engine returns the CLI Terramate engine.
func (c *CLI) Engine() *engine.Engine { return c.state.engine }

// Printers returns the CLI printers.
func (c *CLI) Printers() printer.Printers { return c.printers }

// Exec executes the CLI with the given arguments.
func (c *CLI) Exec(args []string) {
	_ = ConfigureLogging(defaultLogLevel, defaultLogFmt, defaultLogDest,
		c.state.stdout, c.state.stderr)

	if len(args) == 0 {
		// WHY: avoid default kong error, print help
		args = []string{"--help"}
	}

	var err error

	c.state.wd, err = os.Getwd()
	if err != nil {
		printer.Stderr.Error(err)
		os.Exit(1)
	}

	kctx, kerr := c.parser.Parse(args)

	if c.kongExit && c.kongExitStatus == 0 {
		// NOTE(i4k): AFAIK this only happens for `terramate --help`.
		return
	}

	var hasRootFlagSet bool
	var rootFlagSet string
	var rootFlagVal any
	var rootFlagRun func(c *CLI, v any) error

	for _, chk := range c.rootFlagCheckers {
		if name, val, run, isSet := chk(c.input, c); isSet {
			hasRootFlagSet = true
			rootFlagSet = name
			rootFlagVal = val
			rootFlagRun = run
			break
		}
	}

	if kerr != nil {
		if strings.HasPrefix(kerr.Error(), "expected one of ") {
			// It falls here when did not provide any command.
			// But we support `terramate --version` (potentially other cases in the future)
			// then we check the root flags here and return successfully if any of them
			// are set.
			if hasRootFlagSet {
				err := rootFlagRun(c, rootFlagVal)
				if err != nil {
					printer.Stderr.Fatal(err)
				}
				return
			}
		}
		printer.Stderr.Fatal(kerr)
	}

	if hasRootFlagSet {
		// NOTE(i4k): this can only if a command is provided together with a root flag.
		// This is a conflict.
		printer.Stderr.Fatal(errors.E("command %s cannot be used with flag %s", kctx.Command(), rootFlagSet))
	}

	ctx := context.WithValue(context.Background(), KongContext, kctx)
	ctx = context.WithValue(ctx, KongError, err)

	cmd, ok, cont, err := c.beforeConfigHandler(ctx, c)
	if err != nil {
		printer.Stderr.Fatal(err)
	}
	if ok {
		err := cmd.Exec(ctx)
		if err != nil {
			printer.Stderr.FatalWithDetails(fmt.Sprintf("executing %q", cmd.Name()), err)
		}
		return
	}

	if !cont {
		return
	}

	engine, foundRoot, err := engine.Load(c.state.wd, c.state.changeDetectionEnabled, c.clicfg, c.state.uimode, c.printers, c.state.verbose, c.hclOptions...)
	if err != nil {
		printer.Stderr.FatalWithDetails("unable to parse configuration", err)
	}

	if !foundRoot {
		printer.Stderr.Fatal(`Terramate was unable to detect a project root.

Please ensure you run Terramate inside a Git repository or create a new one here by calling 'git init'.

Using Terramate together with Git is the recommended way.

Alternatively you can create a Terramate config to make the current directory the project root.

Please see https://terramate.io/docs/cli/configuration/project-setup for details.
`)
	}

	defer c.sendAndWaitForAnalytics()

	c.state.engine = engine
	cmd, found, cont, err := c.afterConfigHandler(ctx, c)
	if err != nil {
		printer.Stderr.Fatal(err)
	}

	if !found && cont {
		return
	}

	if !found {
		panic("command not found -- should be handled by kong")
	}

	err = cmd.Exec(context.TODO())
	if err != nil {
		printer.Stderr.Fatal(err)
	}
}

// InitAnalytics initializes the analytics record.
func (c *CLI) InitAnalytics(cmd string, opts ...tel.MessageOpt) {
	cpsigfile := filepath.Join(c.clicfg.UserTerramateDir, "checkpoint_signature")
	anasigfile := filepath.Join(c.clicfg.UserTerramateDir, "analytics_signature")

	project := c.state.engine.Project()
	var repo *git.Repository
	if project.IsRepo() {
		repo, _ = project.Repo()
	}

	r := tel.DefaultRecord
	r.Set(
		tel.Command(cmd),
		tel.OrgName(c.cloudOrgName()),
		tel.DetectFromEnv(cliauth.CredentialFile(c.clicfg), cpsigfile, anasigfile, project.CIPlatform(), repo),
		tel.StringFlag("chdir", c.state.wd),
	)
	r.Set(opts...)
}

func (c *CLI) sendAndWaitForAnalytics() {
	// There are several ways to disable this, but this requires the least amount of special handling.
	// Prepare the record, but don't send it.
	if !c.isTelemetryEnabled() {
		return
	}

	tel.DefaultRecord.Send(tel.SendMessageParams{
		Timeout: 100 * time.Millisecond,
		Product: c.product,
		Version: c.version,
	})

	if err := tel.DefaultRecord.WaitForSend(); err != nil {
		logger := log.With().
			Str("action", "cli.sendAndWaitForAnalytics()").
			Logger()
		logger.Debug().Err(err).Msgf("failed to wait for analytics")
	}
}

func (c *CLI) isTelemetryEnabled() bool {
	if c.clicfg.DisableTelemetry {
		return false
	}

	cfg := c.state.engine.Config().Tree().Node
	if cfg.Terramate == nil ||
		cfg.Terramate.Config == nil ||
		cfg.Terramate.Config.Telemetry == nil ||
		cfg.Terramate.Config.Telemetry.Enabled == nil {
		return true
	}
	return *cfg.Terramate.Config.Telemetry.Enabled
}

func (c *CLI) cloudOrgName() string {
	orgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if orgName != "" {
		return orgName
	}

	cfg := c.state.engine.Config().Tree().Node
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Cloud != nil {
		return cfg.Terramate.Config.Cloud.Organization
	}

	return ""
}

// ConfigureLogging configures Terramate global logging.
func ConfigureLogging(logLevel, logFmt, logdest string, stdout, stderr io.Writer) error {
	var output io.Writer

	switch logdest {
	case "stdout":
		output = stdout
	case "stderr":
		output = stderr
	default:
		return errors.E("unknown log destination %q", logdest)
	}

	zloglevel, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		zloglevel = zerolog.FatalLevel
	}

	zerolog.SetGlobalLevel(zloglevel)

	switch logFmt {
	case "json":
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(output)
	case "text": // no color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339})
	default: // default: console mode using color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: false, TimeFormat: time.RFC3339})
	}
	return nil
}

func runCheckpoint(version string, clicfg cliconfig.Config, result chan *checkpoint.CheckResponse) {
	if clicfg.DisableCheckpoint {
		result <- nil
		return
	}

	logger := log.With().
		Str("action", "runCheckpoint()").
		Logger()

	cacheFile := filepath.Join(clicfg.UserTerramateDir, "checkpoint_cache")

	var signatureFile string
	if !clicfg.DisableCheckpointSignature {
		signatureFile = filepath.Join(clicfg.UserTerramateDir, "checkpoint_signature")
	}

	resp, err := checkpoint.CheckAt(defaultTelemetryEndpoint(),
		&checkpoint.CheckParams{
			Product:       "terramate",
			Version:       version,
			SignatureFile: signatureFile,
			CacheFile:     cacheFile,
		},
	)
	if err != nil {
		logger.Debug().Msgf("checkpoint error: %v", err)
		resp = nil
	}

	result <- resp
}

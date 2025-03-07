package tui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"os"
	"time"

	_ "embed"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/exit"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/printer"

	tel "github.com/terramate-io/terramate/cmd/terramate/cli/telemetry"
)

// CLI is the Terramate command-line interface opaque type.
// The default flag spec is [input.Spec] and handler is [DefaultAfterConfigHandler].
type CLI struct {
	version string

	clicfg cliconfig.Config
	state  state

	printers printer.Printers

	// flag handling

	kongOpts       kongOptions
	kongExit       bool
	kongExitStatus int

	// The CLI engine works with any spec.
	input  any
	parser *kong.Kong

	beforeConfigHandler Handler
	afterConfigHandler  Handler
}

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
	uimode  UIMode
}

type Option func(*CLI) error

const (
	name = "Terramate"
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
			&Spec{},
			DefaultBeforeConfigHandler,
			DefaultAfterConfigHandler,
			defaultRootFlagCheckers()...)(c)

		if err != nil {
			return nil, err
		}
	}
	c.printers.Stdout = printer.NewPrinter(c.state.stdout)
	c.printers.Stderr = printer.NewPrinter(c.state.stderr)
	return c, nil
}

type contextStr string

const KongContext contextStr = "kong.context"

func (c *CLI) DidKongExit() bool {
	return c.kongExit
}

func (c *CLI) InputSpec() any { return c.input }

func (c *CLI) Version() string { return c.version }

func (c *CLI) WorkingDir() string { return c.state.wd }

func (c *CLI) Config() *config.Root { return c.state.engine.Config() }

func (c *CLI) Engine() *engine.Engine { return c.state.engine }

func (c *CLI) Printers() printer.Printers { return c.printers }

func (c *CLI) Exec(args []string) {
	ConfigureLogging(defaultLogLevel, defaultLogFmt, defaultLogDest,
		c.state.stdout, c.state.stderr)

	kctx, err := c.parser.Parse(args)
	if err != nil {
		printer.Stderr.Error(err)
		os.Exit(1)
	}

	c.state.wd, err = os.Getwd()
	if err != nil {
		printer.Stderr.Error(err)
		os.Exit(1)
	}

	ctx := context.WithValue(context.Background(), KongContext, kctx)
	cmd, ok, cont, err := c.beforeConfigHandler(ctx, c)
	if err != nil {
		printer.Stderr.Error(err)
		os.Exit(1)
	}
	if ok {
		err := cmd.Exec(ctx)
		if err != nil {
			printer.Stderr.ErrorWithDetails(fmt.Sprintf("executing %q", cmd.Name()), err)
			os.Exit(int(errors.ExitStatus(err)))
		}
	} else {
		if !cont {
			os.Exit(0)
		}
	}

	engine, foundRoot, err := engine.Load(c.state.wd, c.printers)
	if err != nil {
		printer.Stderr.ErrorWithDetails("unable to parse configuration", err)
		os.Exit(1)
	}

	if !foundRoot {
		printer.Stderr.Println(`Error: Terramate was unable to detect a project root.

Please ensure you run Terramate inside a Git repository or create a new one here by calling 'git init'.

Using Terramate together with Git is the recommended way.

Alternatively you can create a Terramate config to make the current directory the project root.

Please see https://terramate.io/docs/cli/configuration/project-setup for details.
`)
		os.Exit(1)
	}

	var exitcode exit.Status
	defer func() {
		c.sendAndWaitForAnalytics()
	}()

	c.state.engine = engine
	cmd, found, cont, err := c.afterConfigHandler(ctx, c)
	if err != nil {
		exitcode = errors.ExitStatus(err)
		printer.Stderr.Error(err)
		goto exit
	}

	if !found && cont {
		goto exit
	}

	if !found {
		panic("command not found -- should be handled by kong")
	}

	err = cmd.Exec(context.TODO())
	if err != nil {
		printer.Stderr.Error(err)
		exitcode = errors.ExitStatus(err)
		goto exit
	}

exit:
	os.Exit(int(exitcode))
}

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
		tel.DetectFromEnv(auth.CredentialFile(c.clicfg), cpsigfile, anasigfile, project.CIPlatform(), repo),
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

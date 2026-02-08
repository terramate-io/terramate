// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/commands"
	reqvercmd "github.com/terramate-io/terramate/commands/requiredversion"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
	"github.com/terramate-io/terramate/ui/tui/out"
	"google.golang.org/grpc"

	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

// CLI is the Terramate command-line interface opaque type.
// The default flag spec is [input.Spec] and handler is [DefaultAfterConfigHandler].
type CLI struct {
	product       string
	prettyProduct string
	version       string

	clicfg cliconfig.Config
	state  state

	printers printer.Printers

	// flag handling

	kongOpts       kongOptions
	kongExit       bool
	kongExitStatus int

	// The CLI engine works with any spec.
	input              any
	parser             *kong.Kong
	rootFlagCheckers   []RootFlagHandlers
	hclOptions         []hcl.Option
	kongDynamicOptions []kong.Option

	checkpointResponse chan *checkpoint.CheckResponse

	commandSelector CommandSelector
	pluginCommands  map[string]PluginCommand

	bindings                  *di.Bindings
	beforeConfigSetupHandlers []BindingsSetupHandler
	afterConfigSetupHandlers  []BindingsSetupHandler

	postInitEngineHooks []PostInitEngineHook

	// host service server for plugins
	hostService     *plugingrpc.HostService
	hostServiceStop func()
}

// CommandSelector is a function that handles command selection.
type CommandSelector func(ctx context.Context, c *CLI, command string, flags any) (commands.Command, error)

// PostInitEngineHook is a function that is run after the engine was initialized.
type PostInitEngineHook func(ctx context.Context, c *CLI) error

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
}

// Option is a function that modifies the CLI behavior.
type Option func(*CLI) error

const (
	name       = "terramate"
	prettyName = "Terramate"
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
		product:       name,
		prettyProduct: prettyName,
		version:       terramate.Version(),
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
			SelectCommand,
			DefaultRootFlagHandlers()...)(c)

		if err != nil {
			return nil, err
		}
	}

	c.bindings = di.NewBindings(context.Background())

	if len(c.beforeConfigSetupHandlers) == 0 {
		c.beforeConfigSetupHandlers = []BindingsSetupHandler{
			DefaultBeforeConfigSetup,
		}
	}
	if len(c.afterConfigSetupHandlers) == 0 {
		c.afterConfigSetupHandlers = []BindingsSetupHandler{
			DefaultAfterConfigSetup,
		}
	}

	c.printers.Stdout = printer.NewPrinter(c.state.stdout)
	c.printers.Stderr = printer.NewPrinter(c.state.stderr)
	c.state.uimode = engine.HumanMode

	if val := os.Getenv("CI"); envVarIsSet(val) {
		c.state.uimode = engine.AutomationMode
	}

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

// Product returns the canonical CLI product name.
func (c *CLI) Product() string { return c.product }

// PrettyProduct returns the CLI product name with prettier formatting.
func (c *CLI) PrettyProduct() string { return c.prettyProduct }

// Version returns the CLI version.
func (c *CLI) Version() string { return c.version }

// WorkingDir returns the CLI working directory.
func (c *CLI) WorkingDir() string { return c.state.wd }

// Config returns the CLI Terramate user configuration.
func (c *CLI) Config() cliconfig.Config { return c.clicfg }

// Engine returns the CLI Terramate engine.
func (c *CLI) Engine() *engine.Engine { return c.state.engine }

// Printers returns the CLI printers.
func (c *CLI) Printers() printer.Printers { return c.printers }

// Reload reloads the engine configuration and re-runs post-init hooks.
func (c *CLI) Reload(ctx context.Context) error {
	if c.state.engine == nil {
		return errors.E("engine not initialized: Reload requires EngineRequirement")
	}
	if err := c.state.engine.ReloadConfig(); err != nil {
		return err
	}
	for _, hook := range c.postInitEngineHooks {
		if err := hook(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// Stdout returns the stdout writer.
func (c *CLI) Stdout() io.Writer { return c.state.stdout }

// Stderr returns the stderr writer.
func (c *CLI) Stderr() io.Writer { return c.state.stderr }

// Stdin returns the stdout writer.
func (c *CLI) Stdin() io.Reader { return c.state.stdin }

// ShowForm renders a form and returns the collected values.
func (c *CLI) ShowForm(context.Context, *pb.FormRequest) (*pb.FormResponse, error) {
	return nil, errors.E("form rendering is only supported for plugin commands")
}

func (c *CLI) initLogging(parsedArgs *FlagSpec) error {
	// Called again with parsed parameters.
	err := ConfigureLogging(parsedArgs.LogLevel, parsedArgs.LogFmt,
		parsedArgs.LogDestination, c.state.stdout, c.state.stderr)
	if err != nil {
		return err
	}

	c.state.verbose = parsedArgs.Verbose

	if parsedArgs.Quiet {
		c.state.verbose = -1
	}

	c.state.output = out.New(c.state.verbose, c.state.stdout, c.state.stderr)
	return nil
}

func (c *CLI) loadUserConfig(parsedArgs *FlagSpec) error {
	var err error
	c.clicfg, err = cliconfig.Load()
	if err != nil {
		printer.Stderr.ErrorWithDetails("failed to load cli configuration file", err)
		return errors.E(ErrSetup)
	}

	if parsedArgs.DisableCheckpoint {
		c.clicfg.DisableCheckpoint = parsedArgs.DisableCheckpoint
	}

	if parsedArgs.DisableCheckpointSignature {
		c.clicfg.DisableCheckpointSignature = parsedArgs.DisableCheckpointSignature
	}

	if c.clicfg.UserTerramateDir == "" {
		homeTmDir, err := userTerramateDir()
		if err != nil {
			printer.Stderr.ErrorWithDetails(fmt.Sprintf("Please either export the %s environment variable or "+
				"set the homeTerramateDir option in the %s configuration file",
				cliconfig.DirEnv,
				cliconfig.Filename),
				err)
			return errors.E(ErrSetup)

		}
		c.clicfg.UserTerramateDir = homeTmDir
	}

	return nil
}

func (c *CLI) initCheckpoint() {
	c.checkpointResponse = make(chan *checkpoint.CheckResponse, 1)
	go runCheckpoint(
		c.product,
		c.version,
		c.clicfg,
		c.checkpointResponse,
	)
}

func (c *CLI) setWorkingDirectory(parsedArgs *FlagSpec) error {
	logger := log.With().
		Str("workingDir", c.state.wd).
		Logger()

	var err error
	if parsedArgs.Chdir != "" {
		logger.Debug().
			Str("dir", parsedArgs.Chdir).
			Msg("Changing working directory")

		err = os.Chdir(parsedArgs.Chdir)
		if err != nil {
			return errors.E(ErrSetup, err, "changing working dir to %s", parsedArgs.Chdir)
		}

		c.state.wd, err = os.Getwd()
		if err != nil {
			return errors.E(ErrSetup, err, "getting workdir")
		}
	}

	c.state.wd, err = filepath.EvalSymlinks(c.state.wd)
	if err != nil {
		return errors.E(ErrSetup, err, "evaluating symlinks on working dir: %s", c.state.wd)
	}

	return nil
}

func (c *CLI) initEngine(req *commands.EngineRequirement) error {
	engine, foundRoot, err := engine.Load(c.state.wd, req.LoadTerragruntModules, c.clicfg, c.state.uimode, c.printers, c.state.verbose, c.hclOptions...)
	if err != nil {
		// TODO: This should return the error.
		printer.Stderr.FatalWithDetails("unable to parse configuration", err)
	}

	if !foundRoot {
		// TODO: This should return the error.
		printer.Stderr.Fatal(`Terramate was unable to detect a project root.

Please ensure you run Terramate inside a Git repository or create a new one here by calling 'git init'.

Using Terramate together with Git is the recommended way. Git is required to be installed.

Alternatively you can create a Terramate config to make the current directory the project root.

Please see https://terramate.io/docs/cli/projects/configuration for details.
`)
	}

	c.state.engine = engine

	return nil
}

func (c *CLI) checkEngineInvariants(parsedArgs *FlagSpec) error {
	// Commits
	if parsedArgs.Changed && !c.Engine().Project().HasCommits() {
		return errors.E("flag --changed requires a repository with at least two commits")
	}

	// Required version
	rv := reqvercmd.Spec{
		Version: c.version,
		Root:    c.state.engine.Config(),
	}

	err := rv.Exec(context.TODO())
	if err != nil {
		return err
	}

	return nil
}

func (c *CLI) checkExperiments(names ...string) {
	cfg := c.state.engine.Config()

	for _, name := range names {

		if cfg.HasExperiment(name) {
			continue
		}

		printer.Stderr.Error(fmt.Sprintf(`The "%s" feature is not enabled`, name))
		printer.Stderr.Println(`In order to enable it you must set the terramate.config.experiments attribute.`)
		printer.Stderr.Println(fmt.Sprintf(`Example:

terramate {
  config {
    experiments = ["%s"]
  }
}`, name))

		// TODO(snk): This shouldn't just exit...
		os.Exit(1)
	}
}

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

	// Parse command line arguments.
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

	// Errors on this level are fatal.
	mustSucceed := func(err error) {
		if err != nil {
			printer.Stderr.Fatal(err)
		}
	}

	parsedArgs := AsFlagSpec[FlagSpec](c.input)
	if parsedArgs == nil {
		panic(errors.E(errors.ErrInternal, "please report this as a bug"))
	}

	migrateFlagAliases(parsedArgs)

	// profiler is only started if Terramate is built with -tags profiler
	startProfiler(parsedArgs.CPUProfiling)
	defer stopProfiler(parsedArgs.CPUProfiling)

	// Setup context.
	ctx := context.Background()
	ctx = context.WithValue(ctx, KongContext, kctx)
	ctx = context.WithValue(ctx, KongError, err)
	ctx = di.WithBindings(ctx, c.bindings)

	// Setup bindings before config loading.
	for _, setup := range c.beforeConfigSetupHandlers {
		mustSucceed(setup(c, c.bindings))
	}
	mustSucceed(di.Validate(c.bindings))
	mustSucceed(di.InitAll(c.bindings))

	mustSucceed(c.initLogging(parsedArgs))
	mustSucceed(c.loadUserConfig(parsedArgs))

	// Start host service server before config parsing so plugins can access it during parsing.
	mustSucceed(c.startHostService())

	c.initCheckpoint()

	// Select the command handler.
	cmd, err := c.commandSelector(ctx, c, kctx.Command(), c.input)
	mustSucceed(err)

	if req, yes := commands.HasRequirement[commands.EngineRequirement](ctx, c, cmd); yes {
		mustSucceed(c.setWorkingDirectory(parsedArgs))

		// Init the engine, this includes loading the config tree.
		mustSucceed(c.initEngine(req))

		// Update host service with the loaded root configuration.
		c.updateHostServiceRoot()
		mustSucceed(c.runPostInitPlugins(ctx))

		mustSucceed(c.checkEngineInvariants(parsedArgs))

		// Experiments require the engine since they are config based.

		if len(req.Experiments) > 0 {
			// TODO(snk): Will os.Exit on fail. This is not nice.
			c.checkExperiments(req.Experiments...)
		}

		c.setProjectAnalytics()

		// Setup bindings after config loading.
		for _, setup := range c.afterConfigSetupHandlers {
			mustSucceed(setup(c, c.bindings))
		}
		mustSucceed(di.Validate(c.bindings))
		mustSucceed(di.InitAll(c.bindings))

		for _, hook := range c.postInitEngineHooks {
			mustSucceed(hook(ctx, c))
		}

		defer c.sendAndWaitForAnalytics()
	}

	// Ensure host service is stopped when CLI exits.
	defer c.stopHostService()

	// Invoke the command handler at last.
	mustSucceed(cmd.Exec(ctx, c))
}

// SetCommandAnalytics initializes the analytics record.
func (c *CLI) SetCommandAnalytics(cmd string, opts ...tel.MessageOpt) {
	allOpts := []tel.MessageOpt{tel.Command(cmd)}
	allOpts = append(allOpts, opts...)

	tel.DefaultRecord.Set(allOpts...)
}

func (c *CLI) startHostService() error {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.E(err, "failed to start host service listener")
	}

	server := grpc.NewServer()
	c.hostService = plugingrpc.NewHostService(nil, c.clicfg.UserTerramateDir)
	pb.RegisterHostServiceServer(server, c.hostService)

	go func() {
		_ = server.Serve(lis)
	}()

	addr := lis.Addr().String()
	if err := os.Setenv(plugingrpc.HostAddrEnv, addr); err != nil {
		server.Stop()
		_ = lis.Close()
		return errors.E(err, "failed to set %s environment variable", plugingrpc.HostAddrEnv)
	}

	c.hostServiceStop = func() {
		server.Stop()
		_ = lis.Close()
	}

	return nil
}

func (c *CLI) stopHostService() {
	if c.hostServiceStop != nil {
		c.hostServiceStop()
		c.hostServiceStop = nil
	}
	c.hostService = nil
}

func (c *CLI) updateHostServiceRoot() {
	if c.hostService == nil || c.state.engine == nil {
		return
	}
	c.hostService.SetRoot(c.state.engine.Config())
}

func (c *CLI) runPostInitPlugins(ctx context.Context) error {
	if isEnvVarSet(os.Getenv("TM_DISABLE_GRPC_PLUGINS")) {
		return nil
	}
	if c.state.engine == nil {
		return nil
	}
	logger := log.With().Str("action", "post-init-plugins").Logger()
	const postInitTimeout = 1 * time.Minute
	installed, err := plugingrpc.DiscoverInstalled(c.clicfg.UserTerramateDir)
	if err != nil {
		return err
	}
	for _, plg := range installed {
		pluginCtx, cancel := context.WithTimeout(ctx, postInitTimeout)
		client, err := plugingrpc.NewHostClient(plg.BinaryPath)
		if err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("failed to start plugin")
			cancel()
			continue
		}
		grpcClient := client.Client()
		caps, err := grpcClient.PluginService.GetCapabilities(pluginCtx, &pb.Empty{})
		if err != nil {
			client.Kill()
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("failed to fetch plugin capabilities")
			cancel()
			continue
		}
		if caps == nil || !caps.HasPostInitHooks {
			client.Kill()
			cancel()
			continue
		}
		resp, err := grpcClient.LifecycleService.PostInit(pluginCtx, &pb.PostInitRequest{
			RootDir: c.state.engine.Config().HostDir(),
		})
		client.Kill()
		cancel()
		if err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("post-init hook failed")
			continue
		}
		if err := plugingrpc.DiagnosticsError(resp.Diagnostics); err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("post-init diagnostics error")
			continue
		}
		if err := c.applyStackUpdates(ctx, resp.StackUpdates); err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("post-init stack update failed")
			continue
		}
		if err := c.applyConfigPatches(plg.Manifest.Name, resp.ConfigPatches); err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("post-init config patch failed")
			continue
		}
	}
	return nil
}

func isEnvVarSet(val string) bool {
	return val != "" && val != "0" && val != "false"
}

func (c *CLI) applyStackUpdates(ctx context.Context, updates []*pb.StackMetadataUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	if c.hostService == nil {
		return errors.E(errors.ErrInternal, "host service is not initialized")
	}
	for _, update := range updates {
		if update == nil {
			continue
		}
		_, err := c.hostService.SetStackMetadata(ctx, &pb.SetStackRequest{
			Path:     update.Path,
			Metadata: update.Metadata,
			Merge:    update.Merge,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) applyConfigPatches(pluginName string, patches []*pb.ConfigPatch) error {
	if len(patches) == 0 {
		return nil
	}
	root := c.state.engine.Config()
	for _, patch := range patches {
		if patch == nil || len(patch.PluginData) == 0 {
			continue
		}
		if err := c.applyConfigPatch(root, pluginName, patch); err != nil {
			return err
		}
	}
	return nil
}

func (c *CLI) applyConfigPatch(root *config.Root, pluginName string, patch *pb.ConfigPatch) error {
	if patch.Path == "" {
		return errors.E(errors.ErrInternal, "config patch path is required")
	}
	prjPath, err := toProjectPath(root.HostDir(), patch.Path)
	if err != nil {
		return err
	}
	node, found := root.Lookup(prjPath)
	if !found {
		return errors.E(errors.ErrInternal, "config path not found: %s", patch.Path)
	}

	ext := node.Node.External
	if ext == nil {
		ext = &plugingrpc.HCLExternalData{Plugins: map[string]map[string][][]byte{}}
		node.Node.External = ext
	}
	typed, ok := ext.(*plugingrpc.HCLExternalData)
	if !ok {
		return errors.E(errors.ErrInternal, "unexpected external data type %T", ext)
	}
	if typed.Plugins == nil {
		typed.Plugins = map[string]map[string][][]byte{}
	}
	if typed.Plugins[pluginName] == nil {
		typed.Plugins[pluginName] = map[string][][]byte{}
	}
	blockType := "post_init"
	if bt := pluginDataBlockType(patch.PluginData); bt != "" {
		blockType = bt
	}
	typed.Plugins[pluginName][blockType] = append(typed.Plugins[pluginName][blockType], patch.PluginData)
	return nil
}

func pluginDataBlockType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var payload struct {
		BlockType string `json:"block_type"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload.BlockType
}

func toProjectPath(rootDir string, path string) (project.Path, error) {
	if path == "" || path == "/" {
		return project.NewPath("/"), nil
	}
	// Distinguish between absolute filesystem paths (which start with rootDir)
	// and project-relative paths (which start with "/" but are not under rootDir).
	if filepath.IsAbs(path) && strings.HasPrefix(path, rootDir) {
		return project.PrjAbsPath(rootDir, path), nil
	}
	if strings.HasPrefix(path, "/") {
		return project.NewPath(path), nil
	}
	return project.NewPath("/" + filepath.ToSlash(path)), nil
}

func (c *CLI) setProjectAnalytics() {
	cpsigfile := filepath.Join(c.clicfg.UserTerramateDir, "checkpoint_signature")
	anasigfile := filepath.Join(c.clicfg.UserTerramateDir, "analytics_signature")

	project := c.state.engine.Project()
	var repo *git.Repository
	if project.IsRepo() {
		repo, _ = project.Repo()
	}

	r := tel.DefaultRecord
	r.Set(
		tel.OrgName(c.state.engine.CloudOrgName()),
		tel.DetectFromEnv(cliauth.CredentialFile(c.clicfg), cpsigfile, anasigfile, project.CIPlatform(), repo),
		tel.StringFlag("chdir", c.state.wd),
	)
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

func runCheckpoint(product, version string, clicfg cliconfig.Config, result chan *checkpoint.CheckResponse) {
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
			Product:       product,
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

// DefaultBeforeConfigSetup sets up the default bindings.
func DefaultBeforeConfigSetup(*CLI, *di.Bindings) error {
	errs := errors.L()
	// Nothing yet.
	return errs.AsError()
}

// DefaultAfterConfigSetup sets up the default bindings.
func DefaultAfterConfigSetup(c *CLI, b *di.Bindings) error {
	errs := errors.L()
	errs.Append(SetupGenerateAPI(c, b))
	return errs.AsError()
}

// SetupGenerateAPI binds generate.API.
func SetupGenerateAPI(_ *CLI, b *di.Bindings) error {
	return di.Bind(b, generate.NewAPI())
}

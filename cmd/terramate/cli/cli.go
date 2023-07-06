// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	stdfmt "fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/errlog"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/versions"

	"github.com/terramate-io/terramate/stack/trigger"
	"github.com/terramate-io/terramate/stdlib"

	stdjson "encoding/json"

	prj "github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/tf"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/json"

	"github.com/alecthomas/kong"
	"github.com/emicklei/dot"

	"github.com/posener/complete"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/stack"
	"github.com/willabides/kongplete"
)

const (
	// ErrCurrentHeadIsOutOfDate indicates the local HEAD revision is outdated.
	ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch"
	// ErrOutdatedGenCodeDetected indicates outdated generated code detected.
	ErrOutdatedGenCodeDetected errors.Kind = "outdated generated code detected"
	// ErrRootCfgInvalidDir indicates that a root configuration was found outside root
	ErrRootCfgInvalidDir errors.Kind = "root config found outside root dir"
)

const (
	defaultRemote        = "origin"
	defaultBranch        = "main"
	defaultBranchBaseRef = "HEAD^"
)

const (
	defaultLogLevel = "warn"
	defaultLogFmt   = "console"
	defaultLogDest  = "stderr"
)

const defaultVendorDir = "/modules"

const terramateUserConfigDir = ".terramate.d"

type cliSpec struct {
	Version        struct{} `cmd:"" help:"Terramate version"`
	VersionFlag    bool     `name:"version" help:"Terramate version"`
	Chdir          string   `short:"C" optional:"true" predictor:"file" help:"Sets working directory"`
	GitChangeBase  string   `short:"B" optional:"true" help:"Git base ref for computing changes"`
	Changed        bool     `short:"c" optional:"true" help:"Filter by changed infrastructure"`
	Tags           []string `optional:"true" sep:"none" help:"Filter stacks by tags. Use \":\" for logical AND and \",\" for logical OR. Example: --tags app:prod filters stacks containing tag \"app\" AND \"prod\". If multiple --tags are provided, an OR expression is created. Example: \"--tags A --tags B\" is the same as \"--tags A,B\""`
	NoTags         []string `optional:"true" sep:"," help:"Filter stacks that do not have the given tags"`
	LogLevel       string   `optional:"true" default:"warn" enum:"disabled,trace,debug,info,warn,error,fatal" help:"Log level to use: 'disabled', 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'"`
	LogFmt         string   `optional:"true" default:"console" enum:"console,text,json" help:"Log format to use: 'console', 'text', or 'json'"`
	LogDestination string   `optional:"true" default:"stderr" enum:"stderr,stdout" help:"Destination of log messages"`
	Quiet          bool     `optional:"false" help:"Disable output"`
	Verbose        int      `short:"v" optional:"true" default:"0" type:"counter" help:"Increase verboseness of output"`

	DisableCheckGitUntracked   bool `optional:"true" default:"false" help:"Disable git check for untracked files"`
	DisableCheckGitUncommitted bool `optional:"true" default:"false" help:"Disable git check for uncommitted files"`

	DisableCheckpoint          bool `optional:"true" default:"false" help:"Disable checkpoint checks for updates"`
	DisableCheckpointSignature bool `optional:"true" default:"false" help:"Disable checkpoint signature"`

	Create struct {
		Path           string   `arg:"" name:"path" predictor:"file" help:"Path of the new stack relative to the working dir"`
		ID             string   `help:"ID of the stack, defaults to UUID"`
		Name           string   `help:"Name of the stack, defaults to stack dir base name"`
		Description    string   `help:"Description of the stack, defaults to the stack name"`
		Import         []string `help:"Add import block for the given path on the stack"`
		After          []string `help:"Add a stack as after"`
		Before         []string `help:"Add a stack as before"`
		IgnoreExisting bool     `help:"If the stack already exists do nothing and don't fail"`
		NoGenerate     bool     `help:"Disable code generation for the newly created stack"`
	} `cmd:"" help:"Creates a stack on the project"`

	Fmt struct {
		Check bool `help:"Lists unformatted files, exit with 0 if all is formatted, 1 otherwise"`
	} `cmd:"" help:"Format all files inside dir recursively"`

	List struct {
		Why bool `help:"Shows the reason why the stack has changed"`
	} `cmd:"" help:"List stacks"`

	Run struct {
		CloudSyncDeployment   bool     `default:"false" help:"Enable synchronization of stack execution with the Terramate Cloud"`
		DisableCheckGenCode   bool     `default:"false" help:"Disable outdated generated code check"`
		DisableCheckGitRemote bool     `default:"false" help:"Disable checking if local default branch is updated with remote"`
		ContinueOnError       bool     `default:"false" help:"Continue executing in other stacks in case of error"`
		NoRecursive           bool     `default:"false" help:"Do not recurse into child stacks"`
		DryRun                bool     `default:"false" help:"Plan the execution but do not execute it"`
		Reverse               bool     `default:"false" help:"Reverse the order of execution"`
		Command               []string `arg:"" name:"cmd" predictor:"file" passthrough:"" help:"Command to execute"`
	} `cmd:"" help:"Run command in the stacks"`

	Generate struct{} `cmd:"" help:"Generate terraform code for stacks"`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`

	Experimental struct {
		Init struct {
			AllTerraform bool `help:"initialize all Terraform directories containing terraform.backend blocks defined"`
			NoGenerate   bool `help:"skip the generation phase"`
		} `cmd:"" help:"Init existing directories with stacks"`

		Clone struct {
			SrcDir  string `arg:"" name:"srcdir" predictor:"file" help:"Path of the stack being cloned"`
			DestDir string `arg:"" name:"destdir" predictor:"file" help:"Path of the new stack"`
		} `cmd:"" help:"Clones a stack"`

		Trigger struct {
			Stack  string `arg:"" name:"stack" predictor:"file" help:"Path of the stack being triggered"`
			Reason string `default:"" name:"reason" help:"Reason for the stack being triggered"`
		} `cmd:"" help:"Triggers a stack"`

		Metadata struct{} `cmd:"" help:"Shows metadata available on the project"`

		Globals struct {
		} `cmd:"" help:"List globals for all stacks"`

		Generate struct {
			Debug struct {
			} `cmd:"" help:"Shows generate debug information"`
		} `cmd:"" help:"Experimental generate commands"`

		RunGraph struct {
			Outfile string `short:"o" predictor:"file" default:"" help:"Output .dot file"`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\""`
		} `cmd:"" help:"Generate a graph of the execution order"`

		RunOrder struct {
			Basedir string `arg:"" optional:"true" help:"Base directory to search stacks"`
		} `cmd:"" help:"Show the topological ordering of the stacks"`

		RunEnv struct {
		} `cmd:"" help:"List run environment variables for all stacks"`

		Vendor struct {
			Download struct {
				Dir       string `short:"d" predictor:"file" default:"" help:"dir to vendor downloaded project"`
				Source    string `arg:"" name:"source" help:"Terraform module source URL, must be Git/Github and should not contain a reference"`
				Reference string `arg:"" name:"ref" help:"Reference of the Terraform module to vendor"`
			} `cmd:"" help:"Downloads a Terraform module and stores it on the project vendor dir"`
		} `cmd:"" help:"Manages vendored Terraform modules"`

		Eval struct {
			Global map[string]string `short:"g" help:"set/override globals. eg.: --global name=<expr>"`
			AsJSON bool              `help:"Outputs the result as a JSON value"`
			Exprs  []string          `arg:"" help:"expressions to be evaluated" name:"expr" passthrough:""`
		} `cmd:"" help:"Eval expression"`

		PartialEval struct {
			Global map[string]string `short:"g" help:"set/override globals. eg.: --global name=<expr>"`
			Exprs  []string          `arg:"" help:"expressions to be partially evaluated" name:"expr" passthrough:""`
		} `cmd:"" help:"Partial evaluate the expressions"`

		GetConfigValue struct {
			Global map[string]string `short:"g" help:"set/override globals. eg.: --global name=<expr>"`
			AsJSON bool              `help:"Outputs the result as a JSON value"`
			Vars   []string          `arg:"" help:"variable to be retrieved" name:"var" passthrough:""`
		} `cmd:"" help:"Get configuration value"`

		Cloud struct {
			Login struct {
			} `cmd:"" help:"login for cloud.terramate.io"`
			Info struct {
			} `cmd:"" help:"cloud information status"`
		} `cmd:"" help:"Terramate Cloud commands"`
	} `cmd:"" help:"Experimental features (may change or be removed in the future)"`
}

// Exec will execute terramate with the provided flags defined on args.
// Only flags should be on the args slice.
//
// Results will be written on stdout, according to the command flags and
// errors/warnings written on stderr. Exec will abort the process with a status
// code different than zero in the case of fatal errors.
//
// Sometimes sub commands may be executed, the provided stdin will be passed to
// then as the sub process stdin.
//
// Each Exec call is completely isolated from each other (no shared state) as
// far as the parameters are not shared between the run calls.
func Exec(
	version string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) {
	configureLogging(defaultLogLevel, defaultLogFmt, defaultLogDest,
		stdout, stderr)
	c := newCLI(version, args, stdin, stdout, stderr)
	c.run()
}

type cli struct {
	version    string
	ctx        *kong.Context
	parsedArgs *cliSpec
	clicfg     cliconfig.Config
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	output     out.O
	exit       bool
	prj        project
	httpClient http.Client
	cloud      cloudConfig

	checkpointResults chan *checkpoint.CheckResponse

	tags filter.TagClause
}

func newCLI(version string, args []string, stdin io.Reader, stdout, stderr io.Writer) *cli {
	if len(args) == 0 {
		// WHY: avoid default kong error, print help
		args = []string{"--help"}
	}

	logger := log.With().
		Str("action", "newCli()").
		Logger()

	kongExit := false
	kongExitStatus := 0

	parsedArgs := cliSpec{}
	parser, err := kong.New(&parsedArgs,
		kong.Name("terramate"),
		kong.Description("A tool for managing terraform stacks"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Exit(func(status int) {
			// Avoid kong aborting entire process since we designed CLI as lib
			kongExit = true
			kongExitStatus = status
		}),
		kong.Writers(stdout, stderr),
	)

	if err != nil {
		fatal(err, "creating cli parser")
	}

	kongplete.Complete(parser,
		kongplete.WithPredictor("file", complete.PredictFiles("*")),
	)

	ctx, err := parser.Parse(args)

	if kongExit && kongExitStatus == 0 {
		return &cli{exit: true}
	}

	// When we run terramate --version the kong parser just fails
	// since no subcommand was provided (which is odd..but happens).
	// So we check if the flag for version is present before checking the error.
	if parsedArgs.VersionFlag {
		stdfmt.Println(version)
		return &cli{exit: true}
	}

	if err != nil {
		fatal(err, "parsing cli args %v", args)
	}

	configureLogging(parsedArgs.LogLevel, parsedArgs.LogFmt,
		parsedArgs.LogDestination, stdout, stderr)
	// If we don't re-create the logger after configuring we get some
	// log entries with a mix of default fmt and selected fmt.
	logger = log.With().
		Str("action", "newCli()").
		Logger()

	verbose := parsedArgs.Verbose

	if parsedArgs.Quiet {
		verbose = -1
	}

	output := out.New(verbose, stdout, stderr)

	clicfg, err := cliconfig.Load()
	if err != nil {
		fatal(err, "failed to load cli configuration file")
	}

	// cmdline flags override configuration file.

	if parsedArgs.DisableCheckpoint {
		clicfg.DisableCheckpoint = parsedArgs.DisableCheckpoint
	}

	if parsedArgs.DisableCheckpointSignature {
		clicfg.DisableCheckpointSignature = parsedArgs.DisableCheckpointSignature
	}

	if clicfg.UserTerramateDir == "" {
		homeTmDir, err := userTerramateDir()
		if err != nil {
			output.MsgStdErr("Please either export the %s environment variable or "+
				"set the homeTerramateDir option in the %s configuration file",
				cliconfig.DirEnv,
				cliconfig.Filename)

			fatal(err)
		}
		clicfg.UserTerramateDir = homeTmDir
	}

	checkpointResults := make(chan *checkpoint.CheckResponse, 1)
	go runCheckpoint(
		version,
		clicfg,
		checkpointResults,
	)

	switch ctx.Command() {
	case "version":
		logger.Debug().Msg("Get terramate version with version subcommand.")
		stdfmt.Println(version)

		info := <-checkpointResults

		if info != nil {
			if info.Outdated {
				releaseDate := time.Unix(int64(info.CurrentReleaseDate), 0).UTC()
				output.MsgStdOut("\nYour version of Terramate is out of date! The latest version\n"+
					"is %s (released on %s).\nYou can update by downloading from %s",
					info.CurrentVersion, releaseDate.Format(time.UnixDate),
					info.CurrentDownloadURL)
			}

			if len(info.Alerts) > 0 {
				plural := ""
				if len(info.Alerts) > 1 {
					plural = "s"
				}

				output.MsgStdOut("\nYour version of Terramate has %d alert%s:\n", len(info.Alerts), plural)

				for _, alert := range info.Alerts {
					urlDesc := ""
					if alert.URL != "" {
						urlDesc = stdfmt.Sprintf(" (more information at %s)", alert.URL)
					}
					output.MsgStdOut("\t- [%s] %s%s", alert.Level, alert.Message, urlDesc)
				}
			}
		}

		return &cli{exit: true}
	case "install-completions":
		logger.Debug().Msg("Handle `install-completions` command.")

		err := parsedArgs.InstallCompletions.Run(ctx)
		if err != nil {
			fatal(err, "installing shell completions")
		}
		return &cli{exit: true}
	case "experimental cloud login":
		err := googleLogin(output, clicfg)
		if err != nil {
			fatal(err, "authentication failed")
		}
		output.MsgStdOut("authenticated successfully")
		return &cli{exit: true}
	}

	wd, err := os.Getwd()
	if err != nil {
		fatal(err, "getting workdir")
	}

	logger = logger.With().
		Str("workingDir", wd).
		Logger()

	if parsedArgs.Chdir != "" {
		logger.Debug().
			Str("dir", parsedArgs.Chdir).
			Msg("Changing working directory")
		err = os.Chdir(parsedArgs.Chdir)
		if err != nil {
			fatal(err, "changing working dir to %s", parsedArgs.Chdir)
		}

		wd, err = os.Getwd()
		if err != nil {
			fatal(err, "getting workdir: %s")
		}
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		log.Fatal().Msgf("evaluating symlinks on working dir: %s", wd)
	}

	logger.Trace().Msg("Running in directory")

	prj, foundRoot, err := lookupProject(wd)
	if err != nil {
		fatal(err, "looking up project root")
	}

	if !foundRoot {
		log.Fatal().Msg("Project root not found. If you invoke Terramate inside a Git repository, Terramate will automatically assume the top level of your repository as the project root. If you use Terramate in a directory that isn't a Git repository, you must configure the project root by creating a terramate.tm.hcl configuration in the directory you wish to be the top-level of your Terramate project. For details please see https://terramate.io/docs/cli/configuration/project-config#project-configuration.")
	}

	logger.Trace().Msg("Set defaults from parsed command line arguments.")

	err = prj.setDefaults()
	if err != nil {
		fatal(err, "setting configuration")
	}

	if parsedArgs.Changed && !prj.isRepo {
		log.Fatal().Msg("flag --changed provided but no git repository found")
	}

	return &cli{
		version:    version,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		output:     output,
		parsedArgs: &parsedArgs,
		clicfg:     clicfg,
		ctx:        ctx,
		prj:        prj,

		// in order to reduce the number of TCP/SSL handshakes we reuse the same
		// http.Client in all requests, for most hosts.
		// The transport can be tuned here, if needed.
		httpClient:        http.Client{},
		checkpointResults: make(chan *checkpoint.CheckResponse, 1),
	}
}

func (c *cli) run() {
	if c.exit {
		// WHY: parser called exit but with no error (like help)
		return
	}

	logger := log.With().
		Str("action", "run()").
		Str("cmd", c.ctx.Command()).
		Str("workingDir", c.wd()).
		Logger()

	c.checkVersion()
	c.setupFilterTags()

	logger.Debug().Msg("Handle command.")

	switch c.ctx.Command() {
	case "fmt":
		c.format()
	case "create <path>":
		c.createStack()
	case "list":
		c.setupGit()
		c.printStacks()
	case "run":
		log.Fatal().Msg("no command specified")
	case "run <cmd>":
		c.setupGit()
		c.runOnStacks()
	case "generate":
		c.generate()
	case "experimental init":
		c.initStacks()
	case "experimental clone <srcdir> <destdir>":
		c.cloneStack()
	case "experimental trigger <stack>":
		c.triggerStack()
	case "experimental vendor download <source> <ref>":
		c.vendorDownload()
	case "experimental globals":
		c.setupGit()
		c.printStacksGlobals()
	case "experimental generate debug":
		c.setupGit()
		c.generateDebug()
	case "experimental metadata":
		c.setupGit()
		c.printMetadata()
	case "experimental run-graph":
		c.setupGit()
		c.generateGraph()
	case "experimental run-order":
		c.setupGit()
		c.printRunOrder()
	case "experimental run-env":
		c.setupGit()
		c.printRunEnv()
	case "experimental eval":
		log.Fatal().Msg("no expression specified")
	case "experimental eval <expr>":
		c.eval()
	case "experimental partial-eval":
		log.Fatal().Msg("no expression specified")
	case "experimental partial-eval <expr>":
		c.partialEval()
	case "experimental get-config-value":
		log.Fatal().Msg("no variable specified")
	case "experimental get-config-value <var>":
		c.getConfigValue()
	case "experimental cloud info":
		c.cloudInfo()
	default:
		log.Fatal().Msg("unexpected command sequence")
	}
}

func (c *cli) setupGit() {
	logger := log.With().
		Str("action", "setupGit()").
		Str("workingDir", c.wd()).
		Logger()

	if c.prj.isRepo && c.parsedArgs.Changed {
		logger.Trace().Msg("Check git default remote.")

		if err := c.prj.checkDefaultRemote(); err != nil {
			fatal(err, "checking git default remote")
		}

		if c.parsedArgs.GitChangeBase != "" {
			c.prj.baseRef = c.parsedArgs.GitChangeBase
		} else {
			c.prj.baseRef = c.prj.defaultBaseRef()
		}
	}
}

func (c *cli) vendorDownload() {
	source := c.parsedArgs.Experimental.Vendor.Download.Source
	ref := c.parsedArgs.Experimental.Vendor.Download.Reference

	logger := log.With().
		Str("workingDir", c.wd()).
		Str("rootdir", c.rootdir()).
		Str("action", "cli.vendor()").
		Str("source", source).
		Str("ref", ref).
		Logger()

	parsedSource, err := tf.ParseSource(source)
	if err != nil {
		log.Fatal().Msgf("parsing module source %s: %s", source, err)
	}
	if parsedSource.Ref != "" {
		log.Fatal().Msgf("module source %s should not contain a reference", source)
	}
	parsedSource.Ref = ref

	eventsStream := download.NewEventStream()
	eventsHandled := c.handleVendorProgressEvents(eventsStream)

	logger.Debug().Msg("vendoring")

	report := download.Vendor(c.rootdir(), c.vendorDir(), parsedSource, eventsStream)

	logger.Debug().Msg("finished vendoring, waiting for all vendor events to be handled")

	close(eventsStream)
	<-eventsHandled

	logger.Debug().Msg("vendor events handled, creating final report")

	if report.Error != nil {
		if errs, ok := report.Error.(*errors.List); ok {
			for _, err := range errs.Errors() {
				logger.Error().Err(err).Send()
			}
		} else {
			logger.Error().Err(report.Error).Send()
		}
	}

	c.output.MsgStdOut(report.String())
}

func (c *cli) handleVendorProgressEvents(eventsStream download.ProgressEventStream) <-chan struct{} {
	eventsHandled := make(chan struct{})

	go func() {
		for event := range eventsStream {
			c.output.MsgStdOut("vendor: %s %s at %s",
				event.Message, event.Module.Raw, event.TargetDir)
			log.Info().
				Str("module", event.Module.Raw).
				Stringer("vendorDir", event.TargetDir).
				Msg(event.Message)
		}
		close(eventsHandled)
	}()

	return eventsHandled
}

func (c *cli) vendorDir() prj.Path {
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("rootdir", c.rootdir()).
		Str("action", "cli.vendorDir()").
		Logger()

	logger.Trace().Msg("checking vendor dir configuration")

	if c.parsedArgs.Experimental.Vendor.Download.Dir != "" {
		logger.Trace().Msg("using CLI config")

		dir := c.parsedArgs.Experimental.Vendor.Download.Dir
		if !path.IsAbs(dir) {
			dir = prj.PrjAbsPath(c.rootdir(), c.wd()).Join(dir).String()
		}
		return prj.NewPath(dir)
	}

	checkVendorDir := func(dir string) prj.Path {
		if !path.IsAbs(dir) {
			log.Fatal().Msgf("vendorDir %s defined is not an absolute path", dir)
		}
		return prj.NewPath(dir)
	}

	dotTerramate := filepath.Join(c.rootdir(), ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {
		logger.Trace().Msg("no CLI config, checking .terramate")

		cfg, err := hcl.ParseDir(c.rootdir(), filepath.Join(c.rootdir(), ".terramate"))
		if err != nil {
			fatal(err, "parsing vendor dir configuration on .terramate")
		}

		if hasVendorDirConfig(cfg) {
			logger.Trace().Msg("using .terramate config")

			return checkVendorDir(cfg.Vendor.Dir)
		}
	}

	logger.Trace().Msg("no .terramate config, checking root")

	hclcfg := c.rootNode()
	if hasVendorDirConfig(hclcfg) {
		logger.Trace().Msg("using root config")

		return checkVendorDir(hclcfg.Vendor.Dir)
	}

	logger.Trace().Msg("no configuration provided, fallback to default")

	return prj.NewPath(defaultVendorDir)
}

func hasVendorDirConfig(cfg hcl.Config) bool {
	return cfg.Vendor != nil && cfg.Vendor.Dir != ""
}

func (c *cli) triggerStack() {
	stack := c.parsedArgs.Experimental.Trigger.Stack
	reason := c.parsedArgs.Experimental.Trigger.Reason
	if reason == "" {
		reason = "Created using Terramate CLI without setting specific reason."
	}
	logger := log.With().
		Str("stack", stack).
		Logger()

	logger.Debug().Msg("creating stack trigger")

	if !path.IsAbs(stack) {
		stack = filepath.Join(c.wd(), filepath.FromSlash(stack))
	} else {
		stack = filepath.Join(c.rootdir(), filepath.FromSlash(stack))
	}

	stack = filepath.Clean(stack)

	if tmp, err := filepath.EvalSymlinks(stack); err != nil || tmp != stack {
		errlog.Fatal(logger, errors.E("symlinks are disallowed in the stack path"))
	}

	if !strings.HasPrefix(stack, c.rootdir()) {
		errlog.Fatal(logger, errors.E("stack %s is outside project", stack))
	}

	stackPath := prj.PrjAbsPath(c.rootdir(), stack)
	if err := trigger.Create(c.cfg(), stackPath, reason); err != nil {
		errlog.Fatal(logger, err)
	}

	c.output.MsgStdOut("Created trigger for stack %q", stackPath)
}

func (c *cli) cloneStack() {
	srcstack := c.parsedArgs.Experimental.Clone.SrcDir
	deststack := c.parsedArgs.Experimental.Clone.DestDir
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("action", "cli.cloneStack()").
		Str("src", srcstack).
		Str("dest", deststack).
		Logger()

	logger.Trace().Msg("cloning stack")

	srcdir := filepath.Join(c.wd(), srcstack)
	destdir := filepath.Join(c.wd(), deststack)

	if err := stack.Clone(c.cfg(), destdir, srcdir); err != nil {
		fatal(err, "cloning %s to %s", srcstack, deststack)
	}

	c.output.MsgStdOut("Cloned stack %s to %s with success", srcstack, deststack)
	c.output.MsgStdOut("Generating code on the new cloned stack")

	c.generate()
}

func (c *cli) generate() {
	report, vendorReport := c.gencodeWithVendor()

	c.output.MsgStdOut(report.Full())

	vendorReport.RemoveIgnoredByKind(download.ErrAlreadyVendored)

	if !vendorReport.IsEmpty() {
		c.output.MsgStdOut(vendorReport.String())
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		os.Exit(1)
	}
}

// gencodeWithVendor will generate code for the whole project providing automatic
// vendoring of all tm_vendor calls.
func (c *cli) gencodeWithVendor() (generate.Report, download.Report) {
	vendorProgressEvents := download.NewEventStream()
	progressHandlerDone := c.handleVendorProgressEvents(vendorProgressEvents)

	vendorRequestEvents := make(chan event.VendorRequest)
	vendorReports := download.HandleVendorRequests(
		c.prj.rootdir,
		vendorRequestEvents,
		vendorProgressEvents,
	)

	mergedVendorReport := download.MergeVendorReports(vendorReports)

	log.Debug().Msg("generating code")

	report := generate.Do(c.cfg(), c.vendorDir(), vendorRequestEvents)

	log.Debug().Msg("code generation finished, waiting for vendor requests to be handled")

	close(vendorRequestEvents)

	log.Debug().Msg("waiting for vendor report merging")

	vendorReport := <-mergedVendorReport

	log.Debug().Msg("waiting for all progress events")

	close(vendorProgressEvents)
	<-progressHandlerDone

	log.Debug().Msg("all handlers stopped, generating final report")

	return report, vendorReport
}

func (c *cli) checkGitUntracked() bool {
	if c.parsedArgs.DisableCheckGitUntracked {
		return false
	}

	if disableCheck, ok := os.LookupEnv("TM_DISABLE_CHECK_GIT_UNTRACKED"); ok {
		if envVarIsSet(disableCheck) {
			return false
		}
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Git != nil {
		return cfg.Terramate.Config.Git.CheckUntracked
	}

	return true
}

func (c *cli) checkGitUncommited() bool {
	if c.parsedArgs.DisableCheckGitUncommitted {
		return false
	}

	if disableCheck, ok := os.LookupEnv("TM_DISABLE_CHECK_GIT_UNCOMMITTED"); ok {
		if envVarIsSet(disableCheck) {
			return false
		}
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Git != nil {
		return cfg.Terramate.Config.Git.CheckUncommitted
	}

	return true
}

func debugFiles(files []string, msg string) {
	for _, file := range files {
		log.Debug().
			Str("file", file).
			Msg(msg)
	}
}

func (c *cli) gitFileSafeguards(shouldAbort bool) {
	if c.parsedArgs.Run.DryRun {
		return
	}

	debugFiles(c.prj.git.repoChecks.UntrackedFiles, "untracked file")
	debugFiles(c.prj.git.repoChecks.UncommittedFiles, "uncommitted file")

	if c.checkGitUntracked() && len(c.prj.git.repoChecks.UntrackedFiles) > 0 {
		const msg = "repository has untracked files"
		if shouldAbort {
			log.Fatal().Msg(msg)
		} else {
			log.Warn().Msg(msg)
		}
	}

	if c.checkGitUncommited() && len(c.prj.git.repoChecks.UncommittedFiles) > 0 {
		const msg = "repository has uncommitted files"
		if shouldAbort {
			log.Fatal().Msg(msg)
		} else {
			log.Warn().Msg(msg)
		}
	}
}

func (c *cli) gitSafeguardDefaultBranchIsReachable() {
	logger := log.With().
		Bool("is_repository", c.prj.isRepo).
		Bool("is_enabled", c.gitSafeguardRemoteEnabled()).
		Logger()

	if !c.prj.isRepo || !c.gitSafeguardRemoteEnabled() {
		logger.Debug().Msg("Safeguard default-branch-is-reachable is disabled.")
		return
	}

	if err := c.prj.checkRemoteDefaultBranchIsReachable(); err != nil {
		logger.Trace().Bool("is_reachable", false).Err(err).
			Msg("Safeguard default-branch-is-reachable failed.")
		fatal(err)
	}
	logger.Trace().Bool("is_reachable", true).
		Msg("Safeguard default-branch-is-reachable passed.")
}

func (c *cli) listStacks(mgr *stack.Manager, isChanged bool) (*stack.Report, error) {
	if isChanged {
		log.Trace().
			Str("action", "listStacks()").
			Str("workingDir", c.wd()).
			Msg("`Changed` flag was set. List changed stacks.")
		return mgr.ListChanged()
	}
	return mgr.List()
}

func (c *cli) initStacks() {
	if !c.parsedArgs.Experimental.Init.AllTerraform {
		fatal(errors.E("The --all-terraform is required"))
	}

	err := c.initDir(c.wd())
	if err != nil {
		fatal(err, "failed to initialize some directories")
	}

	if c.parsedArgs.Experimental.Init.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return
	}

	root, err := config.LoadRoot(c.rootdir())
	if err != nil {
		fatal(err, "reloading the configuration")
	}

	c.prj.root = *root

	report, vendorReport := c.gencodeWithVendor()
	if report.HasFailures() {
		c.output.MsgStdOut("Code generation failed")
		c.output.MsgStdOut(report.Minimal())
	}

	if vendorReport.HasFailures() {
		c.output.MsgStdOut(vendorReport.String())
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		os.Exit(1)
	}

	c.output.MsgStdOutV(report.Full())
	c.output.MsgStdOutV(vendorReport.String())
}

func (c *cli) initDir(baseDir string) error {
	logger := log.With().
		Str("dir", baseDir).
		Str("action", "cli.initDir()").
		Logger()

	pdir := prj.PrjAbsPath(c.rootdir(), baseDir)
	var isStack bool
	tree, found := c.prj.root.Lookup(pdir)
	if found {
		isStack = tree.IsStack()
	}

	logger.Trace().Msg("scanning TF files")

	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		fatal(errors.E(err, "listing directory entries"))
	}

	errs := errors.L()
	for _, f := range dirs {
		path := filepath.Join(baseDir, f.Name())
		if strings.HasPrefix(f.Name(), ".") {
			logger.Trace().Msgf("ignoring file %s", path)
			continue
		}

		if f.IsDir() {
			errs.Append(c.initDir(path))
			continue
		}

		if isStack {
			continue
		}

		if filepath.Ext(f.Name()) != ".tf" {
			logger.Trace().Msgf("ignoring file %s", path)
			continue
		}

		found, err := tf.IsStack(path)
		if err != nil {
			fatal(errors.E(err, "parsing terraform"))
		}

		if !found {
			logger.Trace().Msgf("ignoring file %s", path)
			continue
		}

		stackDir := baseDir
		stackID, err := uuid.NewRandom()
		if err != nil {
			fatal(err, "creating stack UUID")
		}
		stackSpec := config.Stack{
			Dir: prj.PrjAbsPath(c.rootdir(), stackDir),
			ID:  stackID.String(),
		}

		err = stack.Create(c.cfg(), stackSpec)
		if err != nil {
			errs.Append(err)
			continue
		}

		log.Info().Msgf("created stack %s", stackSpec.Dir)
		c.output.MsgStdOut("Created stack %s", stackSpec.Dir)

		// so other files in the same directory do not trigger stack creation.
		isStack = true
	}
	return errs.AsError()
}

func (c *cli) createStack() {
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("action", "cli.createStack()").
		Str("imports", stdfmt.Sprint(c.parsedArgs.Create.Import)).
		Str("after", stdfmt.Sprint(c.parsedArgs.Create.After)).
		Str("before", stdfmt.Sprint(c.parsedArgs.Create.Before)).
		Logger()

	logger.Trace().Msg("creating stack")

	stackHostDir := filepath.Join(c.wd(), c.parsedArgs.Create.Path)

	stackID := c.parsedArgs.Create.ID
	if stackID == "" {
		logger.Trace().Msg("no ID provided, generating one")

		id, err := uuid.NewRandom()
		if err != nil {
			fatal(err, "creating stack UUID")
		}
		stackID = id.String()
	}

	stackName := c.parsedArgs.Create.Name
	if stackName == "" {
		stackName = filepath.Base(stackHostDir)
	}

	stackDescription := c.parsedArgs.Create.Description
	if stackDescription == "" {
		stackDescription = stackName
	}

	stackSpec := config.Stack{
		Dir:         prj.PrjAbsPath(c.rootdir(), stackHostDir),
		ID:          stackID,
		Name:        stackName,
		Description: stackDescription,
		After:       c.parsedArgs.Create.After,
		Before:      c.parsedArgs.Create.Before,
		Tags:        c.parsedArgs.Tags,
	}

	err := stack.Create(c.cfg(), stackSpec, c.parsedArgs.Create.Import...)
	if err != nil {
		logger := log.With().
			Stringer("stack", stackSpec.Dir).
			Logger()

		if c.parsedArgs.Create.IgnoreExisting &&
			(errors.IsKind(err, stack.ErrStackAlreadyExists) ||
				errors.IsKind(err, stack.ErrStackDefaultCfgFound)) {
			logger.Debug().Msg("stack already exists, ignoring")
			return
		}

		if errors.IsKind(err, stack.ErrStackDefaultCfgFound) {
			logger = logger.With().
				Str("file", stack.DefaultFilename).
				Logger()
		}

		errlog.Fatal(logger, err, "can't create stack")
	}

	log.Info().Msgf("created stack %s", stackSpec.Dir)
	c.output.MsgStdOut("Created stack %s", stackSpec.Dir)

	if c.parsedArgs.Create.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return
	}

	err = c.prj.root.LoadSubTree(stackSpec.Dir)
	if err != nil {
		fatal(err, "loading newly created stack")
	}

	report, vendorReport := c.gencodeWithVendor()
	if report.HasFailures() {
		c.output.MsgStdOut("Code generation failed")
		c.output.MsgStdOut(report.Minimal())
	}

	if vendorReport.HasFailures() {
		c.output.MsgStdOut(vendorReport.String())
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		os.Exit(1)
	}

	c.output.MsgStdOutV(report.Minimal())
	c.output.MsgStdOutV(vendorReport.String())
}

func (c *cli) format() {
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("action", "format()").
		Logger()

	logger.Trace().Msg("formatting all files recursively")
	results, err := fmt.FormatTree(c.wd())
	if err != nil {
		fatal(err, "formatting files")
	}

	logger.Trace().Msg("listing formatted files")
	for _, res := range results {
		path := strings.TrimPrefix(res.Path(), c.wd()+string(filepath.Separator))
		c.output.MsgStdOut(path)
	}

	if c.parsedArgs.Fmt.Check {
		logger.Trace().Msg("checking if we have unformatted files")
		if len(results) > 0 {
			logger.Trace().Msg("we have unformatted files")
			os.Exit(1)
		}
		logger.Trace().Msg("all files formatted, nothing else to do")
		return
	}

	logger.Trace().Msg("saving formatted files")

	errs := errors.L()
	for _, res := range results {
		logger := log.With().
			Str("workingDir", c.wd()).
			Str("filepath", res.Path()).
			Str("action", "format()").
			Logger()
		logger.Trace().Msg("saving formatted file")
		errs.Append(res.Save())
	}

	if err := errs.AsError(); err != nil {
		fatal(err, "saving files formatted files")
	}
}

func (c *cli) printStacks() {
	if c.parsedArgs.List.Why && !c.parsedArgs.Changed {
		log.Fatal().Msg("the --why flag must be used together with --changed")
	}

	mgr := stack.NewManager(c.cfg(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		fatal(err, "listing stacks")
	}

	c.prj.git.repoChecks = report.Checks
	c.gitFileSafeguards(false)

	for _, entry := range c.filterStacks(report.Stacks) {
		stack := entry.Stack

		log.Debug().Msgf("printing stack %s", stack.Dir)

		stackRepr, ok := c.friendlyFmtDir(stack.Dir.String())
		if !ok {
			continue
		}

		if c.parsedArgs.List.Why {
			c.output.MsgStdOut("%s - %s", stackRepr, entry.Reason)
		} else {
			c.output.MsgStdOut(stackRepr)
		}
	}
}

func (c *cli) printRunEnv() {
	mgr := stack.NewManager(c.cfg(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		fatal(err, "listing stacks")
	}

	for _, stackEntry := range c.filterStacks(report.Stacks) {
		envVars, err := run.LoadEnv(c.cfg(), stackEntry.Stack)
		if err != nil {
			fatal(err, "loading stack run environment")
		}

		c.output.MsgStdOut("\nstack %q:", stackEntry.Stack.Dir)

		for _, envVar := range envVars {
			c.output.MsgStdOut("\t%s", envVar)
		}
	}
}

func (c *cli) generateGraph() {
	var getLabel func(s *config.Stack) string

	logger := log.With().
		Str("action", "generateGraph()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().Msg("Handle graph label command line argument.")

	switch c.parsedArgs.Experimental.RunGraph.Label {
	case "stack.name":
		logger.Debug().Msg("Set label to stack name.")

		getLabel = func(s *config.Stack) string { return s.Name }
	case "stack.dir":
		logger.Debug().Msg("Set label stack directory.")

		getLabel = func(s *config.Stack) string { return s.Dir.String() }
	default:
		logger.Fatal().
			Msg("-label expects the values \"stack.name\" or \"stack.dir\"")
	}

	entries, err := stack.List(c.cfg().Tree())
	if err != nil {
		fatal(err, "listing stacks to build graph")
	}

	logger.Debug().Msg("Create new graph.")

	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New()

	visited := dag.Visited{}
	for _, e := range c.filterStacksByWorkingDir(entries) {
		if _, ok := visited[dag.ID(e.Stack.Dir.String())]; ok {
			continue
		}

		if err := run.BuildDAG(
			graph,
			c.cfg(),
			e.Stack,
			"before",
			func(s config.Stack) []string { return s.Before },
			"after",
			func(s config.Stack) []string { return s.After },
			visited,
		); err != nil {
			fatal(err, "building order tree")
		}
	}

	for _, id := range graph.IDs() {
		val, err := graph.Node(id)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("generating graph")
		}

		generateDot(dotGraph, graph, id, val.(*config.Stack), getLabel)
	}

	logger.Debug().
		Msg("Set output of graph.")
	outFile := c.parsedArgs.Experimental.RunGraph.Outfile
	var out io.Writer
	if outFile == "" {
		logger.Trace().Msg("set output to stdout")

		out = c.stdout
	} else {
		logger.Trace().Msg("set output to file")

		f, err := os.Create(outFile)
		if err != nil {
			logger := log.With().
				Str("path", outFile).
				Logger()
			errlog.Fatal(logger, err, "opening file")
		}

		defer func() {
			if err := f.Close(); err != nil {
				fatal(err, "closing output graph file")
			}
		}()

		out = f
	}

	logger.Debug().
		Msg("Write graph to output.")
	_, err = out.Write([]byte(dotGraph.String()))
	if err != nil {
		logger := log.With().
			Str("path", outFile).
			Logger()

		errlog.Fatal(logger, err, "writing output")
	}
}

func generateDot(
	dotGraph *dot.Graph,
	graph *dag.DAG,
	id dag.ID,
	stackval *config.Stack,
	getLabel func(s *config.Stack) string,
) {
	parent := dotGraph.Node(getLabel(stackval))
	for _, childid := range graph.AncestorsOf(id) {
		val, err := graph.Node(childid)
		if err != nil {
			fatal(err, "generating dot file")
		}
		s := val.(*config.Stack)
		n := dotGraph.Node(getLabel(s))

		edges := dotGraph.FindEdges(parent, n)
		if len(edges) == 0 {
			edge := dotGraph.Edge(parent, n)
			if graph.HasCycle(childid) {
				edge.Attr("color", "red")
				continue
			}
		}

		if graph.HasCycle(childid) {
			continue
		}

		generateDot(dotGraph, graph, childid, s, getLabel)
	}
}

func (c *cli) printRunOrder() {
	logger := log.With().
		Str("action", "printRunOrder()").
		Str("workingDir", c.wd()).
		Logger()

	stacks, err := c.computeSelectedStacks(false)
	if err != nil {
		fatal(err, "computing selected stacks")
	}

	logger.Debug().Msg("Get run order.")
	orderedStacks, reason, err := run.Sort(c.cfg(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal(err, "cycle detected on run order: %s", reason)
		} else {
			fatal(err, "failed to plan execution")
		}
	}

	for _, s := range orderedStacks {
		c.output.MsgStdOut(s.Dir().String())
	}
}

func (c *cli) generateDebug() {
	// TODO(KATCIPIS): When we introduce config defined on root context
	// we need to know blocks that have root context, since they should
	// not be filtered by stack selection.
	stacks, err := c.computeSelectedStacks(false)
	if err != nil {
		fatal(err, "generate debug: selecting stacks")
	}

	selectedStacks := map[prj.Path]struct{}{}
	for _, stack := range stacks {
		log.Debug().Msgf("selected stack: %s", stack.Dir())

		selectedStacks[stack.Dir()] = struct{}{}
	}

	results, err := generate.Load(c.cfg(), c.vendorDir())
	if err != nil {
		fatal(err, "generate debug: loading generated code")
	}

	for _, res := range results {
		if _, ok := selectedStacks[res.Dir]; !ok {
			log.Debug().Msgf("discarding dir %s since it is not a selected stack", res.Dir)
			continue
		}
		if res.Err != nil {
			errmsg := stdfmt.Sprintf("generate debug error on dir %s: %v", res.Dir, res.Err)
			log.Error().Msg(errmsg)
			c.output.MsgStdErr(errmsg)
			continue
		}

		files := make([]generate.GenFile, 0, len(res.Files))
		for _, f := range res.Files {
			if f.Condition() {
				files = append(files, f)
			}
		}

		for _, file := range files {
			filepath := path.Join(res.Dir.String(), file.Label())
			c.output.MsgStdOut("%s origin: %v", filepath, file.Range())
		}
	}
}

func (c *cli) printStacksGlobals() {
	logger := log.With().
		Str("action", "printStacksGlobals()").
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")

	mgr := stack.NewManager(c.cfg(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		fatal(err, "listing stacks globals: listing stacks")
	}

	for _, stackEntry := range c.filterStacks(report.Stacks) {
		stack := stackEntry.Stack
		report := globals.ForStack(c.cfg(), stack)
		if err := report.AsError(); err != nil {
			logger := log.With().
				Stringer("stack", stack.Dir).
				Logger()

			errlog.Fatal(logger, err, "listing stacks globals: loading stack")
		}

		globalsStrRepr := report.Globals.String()
		if globalsStrRepr == "" {
			continue
		}

		c.output.MsgStdOut("\nstack %q:", stack.Dir)
		for _, line := range strings.Split(globalsStrRepr, "\n") {
			c.output.MsgStdOut("\t%s", line)
		}
	}
}

func (c *cli) printMetadata() {
	logger := log.With().
		Str("action", "cli.printMetadata()").
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")

	mgr := stack.NewManager(c.cfg(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		fatal(err, "loading metadata: listing stacks")
	}

	stackEntries := c.filterStacks(report.Stacks)
	if len(stackEntries) == 0 {
		return
	}

	c.output.MsgStdOut("Available metadata:")
	c.output.MsgStdOut("\nproject metadata:")
	c.output.MsgStdOut("\tterramate.stacks.list=%v", c.cfg().Stacks())

	for _, stackEntry := range stackEntries {
		stack := stackEntry.Stack

		logger.Debug().
			Stringer("stack", stack).
			Msg("Print metadata for individual stack.")

		tags := []string{}
		if len(stack.Tags) > 0 {
			tags = stack.Tags
		}
		tagsVal, _ := stdjson.Marshal(tags)

		c.output.MsgStdOut("\nstack %q:", stack.Dir)
		if stack.ID != "" {
			c.output.MsgStdOut("\tterramate.stack.id=%q", stack.ID)
		}
		c.output.MsgStdOut("\tterramate.stack.name=%q", stack.Name)
		c.output.MsgStdOut("\tterramate.stack.description=%q", stack.Description)
		c.output.MsgStdOut("\tterramate.stack.tags=%s", string(tagsVal))
		c.output.MsgStdOut("\tterramate.stack.path.absolute=%q", stack.Dir)
		c.output.MsgStdOut("\tterramate.stack.path.basename=%q", stack.PathBase())
		c.output.MsgStdOut("\tterramate.stack.path.relative=%q", stack.RelPath())
		c.output.MsgStdOut("\tterramate.stack.path.to_root=%q", stack.RelPathToRoot(c.cfg()))
	}
}

func (c *cli) checkGenCode() bool {
	if c.parsedArgs.Run.DisableCheckGenCode {
		return false
	}

	if disableCheck, ok := os.LookupEnv("TM_DISABLE_CHECK_GEN_CODE"); ok {
		if envVarIsSet(disableCheck) {
			return false
		}
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Run != nil {
		return cfg.Terramate.Config.Run.CheckGenCode
	}

	return true
}

func (c *cli) eval() {
	ctx := c.setupEvalContext(c.parsedArgs.Experimental.Eval.Global)
	for _, exprStr := range c.parsedArgs.Experimental.Eval.Exprs {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal(err)
		}
		val, err := ctx.Eval(expr)
		if err != nil {
			fatal(err, "eval %q", exprStr)
		}
		c.outputEvalResult(val, c.parsedArgs.Experimental.Eval.AsJSON)
	}
}

func (c *cli) partialEval() {
	ctx := c.setupEvalContext(c.parsedArgs.Experimental.PartialEval.Global)
	for _, exprStr := range c.parsedArgs.Experimental.PartialEval.Exprs {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal(err)
		}
		newexpr, err := ctx.PartialEval(expr)
		if err != nil {
			fatal(err, "partial eval %q", exprStr)
		}
		c.output.MsgStdOut(string(hclwrite.Format(ast.TokensForExpression(newexpr).Bytes())))
	}
}

func (c *cli) getConfigValue() {
	logger := log.With().
		Str("action", "cli.getConfigValue()").
		Logger()

	ctx := c.setupEvalContext(c.parsedArgs.Experimental.GetConfigValue.Global)
	for _, exprStr := range c.parsedArgs.Experimental.GetConfigValue.Vars {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal(err)
		}

		iteratorTraversal, diags := hhcl.AbsTraversalForExpr(expr)
		if diags.HasErrors() {
			fatal(errors.E(diags), "expected a variable accessor")
		}

		varns := iteratorTraversal.RootName()
		if varns != "terramate" && varns != "global" {
			logger.Fatal().Msg("only terramate and global variables are supported")
		}

		val, err := ctx.Eval(expr)
		if err != nil {
			fatal(err, "evaluating expression: %s", exprStr)
		}

		c.outputEvalResult(val, c.parsedArgs.Experimental.GetConfigValue.AsJSON)
	}
}

func (c *cli) outputEvalResult(val cty.Value, asJSON bool) {
	var data []byte
	if asJSON {
		var err error
		data, err = json.Marshal(val, val.Type())
		if err != nil {
			fatal(err, "converting value %s to json", val.GoString())
		}
	} else {
		if val.Type() == cty.String {
			data = []byte(val.AsString())
		} else {
			tokens := ast.TokensForValue(val)
			data = []byte(hclwrite.Format(tokens.Bytes()))
		}
	}

	c.output.MsgStdOut(string(data))
}

func (c *cli) setupEvalContext(overrideGlobals map[string]string) *eval.Context {
	ctx := eval.NewContext(stdlib.Functions(c.wd()))
	runtime := c.cfg().Runtime()
	if config.IsStack(c.cfg(), c.wd()) {
		st, err := config.LoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatal(err, "setup eval context: loading stack config")
		}
		runtime.Merge(st.RuntimeValues(c.cfg()))
	}

	ctx.SetNamespace("terramate", runtime)

	wdPath := prj.PrjAbsPath(c.rootdir(), c.wd())
	tree, ok := c.cfg().Lookup(wdPath)
	if !ok {
		fatal(errors.E("configuration at %s not found", wdPath))
	}
	exprs, err := globals.LoadExprs(tree)
	if err != nil {
		fatal(err, "loading globals expressions")
	}

	for name, exprStr := range overrideGlobals {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal(errors.E(err, "--global %s=%s is an invalid expresssion", name, exprStr))
		}
		parts := strings.Split(name, ".")
		length := len(parts)
		globalPath := globals.NewGlobalAttrPath(parts[0:length-1], parts[length-1])
		exprs.SetOverride(
			wdPath,
			globalPath,
			expr,
			info.NewRange(c.rootdir(), hhcl.Range{
				Filename: "<eval argument>",
				Start:    hhcl.InitialPos,
				End:      hhcl.InitialPos,
			}),
		)
	}

	_ = exprs.Eval(ctx)
	return ctx
}

func envVarIsSet(val string) bool {
	return val != "0" && val != "false"
}

func (c *cli) checkOutdatedGeneratedCode() {
	logger := log.With().
		Str("action", "checkOutdatedGeneratedCode()").
		Logger()

	if !c.checkGenCode() {
		logger.Trace().Msg("outdated generated code check is disabled")
		return
	}

	logger.Trace().Msg("checking if any stack has outdated code")

	outdatedFiles, err := generate.DetectOutdated(c.cfg(), c.vendorDir())

	if err != nil {
		fatal(err, "failed to check outdated code on project")
	}

	for _, outdated := range outdatedFiles {
		logger.Error().
			Str("filename", outdated).
			Msg("outdated code found")
	}

	if len(outdatedFiles) > 0 {
		logger.Fatal().
			Err(errors.E(ErrOutdatedGenCodeDetected)).
			Msg("please run: 'terramate generate' to update generated code")
	}
}

func (c *cli) gitSafeguardRemoteEnabled() bool {
	if c.parsedArgs.Run.DisableCheckGitRemote {
		return false
	}

	if disableCheck, ok := os.LookupEnv("TM_DISABLE_CHECK_GIT_REMOTE"); ok {
		if envVarIsSet(disableCheck) {
			return false
		}
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Git != nil {
		return cfg.Terramate.Config.Git.CheckRemote
	}

	return true
}

func (c *cli) runOnStacks() {
	logger := log.With().
		Str("action", "runOnStacks()").
		Str("workingDir", c.wd()).
		Logger()

	c.gitSafeguardDefaultBranchIsReachable()

	if len(c.parsedArgs.Run.Command) == 0 {
		logger.Fatal().Msgf("run expects a cmd")
	}

	c.checkOutdatedGeneratedCode()
	c.checkSyncDeployment()

	var stacks config.List[*config.SortableStack]
	if c.parsedArgs.Run.NoRecursive {
		st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatal(err, "loading stack in current directory")
		}

		if !found {
			logger.Fatal().
				Msg("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st.Sortable())
	} else {
		var err error
		stacks, err = c.computeSelectedStacks(true)
		if err != nil {
			fatal(err, "computing selected stacks")
		}
	}

	c.createCloudDeployment(stacks, c.parsedArgs.Run.Command)

	logger.Trace().Msg("Get order of stacks to run command on.")

	orderedStacks, reason, err := run.Sort(c.cfg(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal(err, "cycle detected: %s", reason)
		} else {
			fatal(err, "failed to plan execution")
		}
	}

	if c.parsedArgs.Run.Reverse {
		logger.Trace().Msg("Reversing stacks order.")
		config.ReverseStacks(orderedStacks)
	}

	if c.parsedArgs.Run.DryRun {
		logger.Trace().
			Msg("Do a dry run - get order without actually running command.")
		if len(orderedStacks) > 0 {
			c.output.MsgStdOut("The stacks will be executed using order below:")

			for i, s := range orderedStacks {
				stackdir, _ := c.friendlyFmtDir(s.Dir().String())
				c.output.MsgStdOut("\t%d. %s (%s)", i, s.Name, stackdir)
			}
		} else {
			c.output.MsgStdOut("No stacks will be executed.")
		}

		return
	}

	beforeHook := func(s *config.Stack, cmd string) {
		if !c.parsedArgs.Run.CloudSyncDeployment {
			return
		}
		c.syncCloudDeployment(s, cloud.Running)
	}

	afterHook := func(s *config.Stack, err error) {
		if !c.parsedArgs.Run.CloudSyncDeployment {
			return
		}
		var status cloud.Status
		switch {
		case err == nil:
			status = cloud.OK
		case errors.IsKind(err, run.ErrCanceled):
			status = cloud.Canceled
		case errors.IsKind(err, run.ErrFailed):
			status = cloud.Failed
		default:
			panic(errors.E(errors.ErrInternal, "unexpected run status"))
		}

		c.syncCloudDeployment(s, status)
	}

	err = run.Exec(
		c.cfg(),
		orderedStacks,
		c.parsedArgs.Run.Command,
		c.stdin,
		c.stdout,
		c.stderr,
		c.parsedArgs.Run.ContinueOnError,
		beforeHook,
		afterHook,
	)

	if err != nil {
		fatal(err, "one or more commands failed")
	}
}

func (c *cli) wd() string           { return c.prj.wd }
func (c *cli) rootdir() string      { return c.prj.rootdir }
func (c *cli) cfg() *config.Root    { return &c.prj.root }
func (c *cli) rootNode() hcl.Config { return c.prj.root.Tree().Node }
func (c *cli) cred() credential     { return c.cloud.credential }

func (c *cli) friendlyFmtDir(dir string) (string, bool) {
	return prj.FriendlyFmtDir(c.rootdir(), c.wd(), dir)
}

func (c *cli) computeSelectedStacks(ensureCleanRepo bool) (config.List[*config.SortableStack], error) {
	logger := log.With().
		Str("action", "computeSelectedStacks()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().Msg("Create new terramate manager.")

	mgr := stack.NewManager(c.cfg(), c.prj.baseRef)

	logger.Trace().Msg("Get list of stacks.")

	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return nil, err
	}

	c.prj.git.repoChecks = report.Checks
	c.gitFileSafeguards(ensureCleanRepo)

	logger.Trace().Msg("Filter stacks by working directory.")

	entries := c.filterStacks(report.Stacks)
	stacks := make(config.List[*config.SortableStack], len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack.Sortable()
	}

	stacks, err = mgr.AddWantedOf(stacks)
	if err != nil {
		return nil, errors.E(err, "adding wanted stacks")
	}
	return stacks, nil
}

func (c *cli) filterStacks(stacks []stack.Entry) []stack.Entry {
	return c.filterStacksByTags(c.filterStacksByWorkingDir(stacks))
}

func (c *cli) filterStacksByWorkingDir(stacks []stack.Entry) []stack.Entry {
	relwd := prj.PrjAbsPath(c.rootdir(), c.wd()).String()
	if relwd != "/" {
		relwd += "/"
	}
	filtered := []stack.Entry{}
	for _, e := range stacks {
		stackdir := e.Stack.Dir.String()
		if stackdir != "/" {
			stackdir += "/"
		}

		if strings.HasPrefix(stackdir, relwd) {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

func (c *cli) filterStacksByTags(entries []stack.Entry) []stack.Entry {
	if c.tags.IsEmpty() {
		return entries
	}
	filtered := []stack.Entry{}
	for _, entry := range entries {
		if filter.MatchTags(c.tags, entry.Stack.Tags) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (c cli) checkVersion() {
	logger := log.With().
		Str("action", "cli.checkVersion()").
		Str("root", c.rootdir()).
		Logger()

	logger.Trace().Msg("checking if terramate version satisfies project constraint")

	rootcfg := c.rootNode()
	if rootcfg.Terramate == nil {
		logger.Debug().Msg("project root has no config, skipping version check")
		return
	}

	if rootcfg.Terramate.RequiredVersion == "" {
		logger.Debug().Msg("project root config has no required_version, skipping version check")
		return
	}

	if err := versions.Check(
		c.version,
		rootcfg.Terramate.RequiredVersion,
		rootcfg.Terramate.RequiredVersionAllowPreReleases,
	); err != nil {
		fatal(err)
	}
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

func (c *cli) setupFilterTags() {
	clauses, found, err := filter.ParseTagClauses(c.parsedArgs.Tags...)
	if err != nil {
		fatal(err)
	}
	if found {
		c.tags = clauses
	}

	for _, val := range c.parsedArgs.NoTags {
		err := tag.Validate(val)
		if err != nil {
			fatal(err)
		}
	}
	var noClauses filter.TagClause
	if len(c.parsedArgs.NoTags) == 0 {
		return
	}
	if len(c.parsedArgs.NoTags) == 1 {
		noClauses = filter.TagClause{
			Op:  filter.NEQ,
			Tag: c.parsedArgs.NoTags[0],
		}
	} else {
		var children []filter.TagClause
		for _, tagname := range c.parsedArgs.NoTags {
			children = append(children, filter.TagClause{
				Op:  filter.NEQ,
				Tag: tagname,
			})
		}
		noClauses = filter.TagClause{
			Op:       filter.AND,
			Children: children,
		}
	}

	if c.tags.IsEmpty() {
		c.tags = noClauses
		return
	}

	switch c.tags.Op {
	case filter.AND:
		c.tags.Children = append(c.tags.Children, noClauses)
	default:
		c.tags = filter.TagClause{
			Op:       filter.AND,
			Children: []filter.TagClause{c.tags, noClauses},
		}
	}
}

func newGit(basedir string, checkrepo bool) (*git.Git, error) {
	log.Debug().
		Str("action", "newGit()").
		Msg("Create new git wrapper providing config.")
	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
		Env:        os.Environ(),
	})

	if err != nil {
		return nil, err
	}

	if checkrepo && !g.IsRepository() {
		return nil, errors.E("dir %q is not a git repository", basedir)
	}

	return g, nil
}

func lookupProject(wd string) (prj project, found bool, err error) {
	prj = project{
		wd: wd,
	}

	logger := log.With().
		Str("action", "lookupProject()").
		Str("workingDir", wd).
		Logger()

	logger.Trace().Msg("Create new git wrapper.")

	rootcfg, rootCfgPath, rootfound, err := config.TryLoadConfig(wd)
	if err != nil {
		return project{}, false, err
	}

	gw, err := newGit(wd, false)
	if err == nil {
		logger.Trace().Msg("Get root of git repo.")

		gitdir, err := gw.Root()
		if err == nil {
			logger.Trace().Msg("Get absolute path of git directory.")

			gitabs := gitdir
			if !filepath.IsAbs(gitabs) {
				gitabs = filepath.Join(wd, gitdir)
			}

			logger.Trace().Msg("Evaluate symbolic links.")

			rootdir, err := filepath.EvalSymlinks(gitabs)
			if err != nil {
				return project{}, false, errors.E(err, "failed evaluating symlinks of %q", gitabs)
			}

			if rootfound && strings.HasPrefix(rootCfgPath, rootdir) && rootCfgPath != rootdir {
				log.Warn().
					Str("rootConfig", rootCfgPath).
					Str("projectRoot", rootdir).
					Err(errors.E(ErrRootCfgInvalidDir)).
					Msg("ignoring root config")
			}

			logger.Trace().Msg("Load root config.")

			cfg, err := config.LoadRoot(rootdir)
			if err != nil {
				return project{}, false, err
			}

			prj.isRepo = true
			prj.root = *cfg
			prj.rootdir = rootdir
			prj.git.wrapper = gw

			return prj, true, nil
		}
	}

	if !rootfound {
		return project{}, false, nil
	}

	prj.rootdir = rootCfgPath
	prj.root = *rootcfg
	return prj, true, nil

}

func configureLogging(logLevel, logFmt, logdest string, stdout, stderr io.Writer) {
	var output io.Writer

	switch logdest {
	case "stdout":
		output = stdout
	case "stderr":
		output = stderr
	default:
		log.Fatal().Msgf("unknown log destination %q", logdest)
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
}

func fatal(err error, args ...any) {
	errlog.Fatal(log.Logger, err, args...)
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdfmt "fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate/cloud"
	cloudstack "github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/safeguard"
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

const (
	// HumanMode is the default normal mode when Terramate is executed at the user's machine.
	HumanMode UIMode = iota
	// AutomationMode is the mode when Terramate executes in the CI/CD environment.
	AutomationMode
)

// UIMode defines different modes of operation for the cli.
type UIMode int

type kongParallelFlag struct {
	Value int
}

const defaultParallelRunCount = 5

type cliSpec struct {
	Version        struct{} `cmd:"" help:"Terramate version"`
	VersionFlag    bool     `name:"version" help:"Terramate version"`
	Chdir          string   `short:"C" optional:"true" predictor:"file" help:"Sets working directory"`
	GitChangeBase  string   `short:"B" optional:"true" help:"Git base ref for computing changes"`
	Changed        bool     `short:"c" optional:"true" help:"Filter by changed infrastructure"`
	Tags           []string `optional:"true" sep:"none" help:"Filter stacks by tags. Use \":\" for logical AND and \",\" for logical OR. Example: --tags app:prod filters stacks containing tag \"app\" AND \"prod\". If multiple --tags are provided, an OR expression is created. Example: \"--tags a --tags b\" is the same as \"--tags a,b\""`
	NoTags         []string `optional:"true" sep:"," help:"Filter stacks that do not have the given tags"`
	LogLevel       string   `optional:"true" default:"warn" enum:"disabled,trace,debug,info,warn,error,fatal" help:"Log level to use: 'disabled', 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'"`
	LogFmt         string   `optional:"true" default:"console" enum:"console,text,json" help:"Log format to use: 'console', 'text', or 'json'"`
	LogDestination string   `optional:"true" default:"stderr" enum:"stderr,stdout" help:"Destination of log messages"`
	Quiet          bool     `optional:"false" help:"Disable output"`
	Verbose        int      `short:"v" optional:"true" default:"0" type:"counter" help:"Increase verboseness of output"`

	deprecatedGlobalSafeguardsCliSpec

	// DEPRECATED
	DisableCheckpoint          bool `hidden:"true" optional:"true" default:"false" help:"Disable checkpoint checks for updates"`
	DisableCheckpointSignature bool `hidden:"true" optional:"true" default:"false" help:"Disable checkpoint signature"`

	Create struct {
		Path           string   `arg:"" optional:"" name:"path" predictor:"file" help:"Path of the new stack relative to the working dir"`
		ID             string   `help:"ID of the stack, defaults to UUID"`
		Name           string   `help:"Name of the stack, defaults to stack dir base name"`
		Description    string   `help:"Description of the stack, defaults to the stack name"`
		Import         []string `help:"Add import block for the given path on the stack"`
		After          []string `help:"Add a stack as after"`
		Before         []string `help:"Add a stack as before"`
		IgnoreExisting bool     `help:"If the stack already exists do nothing and don't fail"`
		AllTerraform   bool     `help:"initialize all Terraform directories containing terraform.backend blocks defined"`
		EnsureStackIds bool     `help:"generate an UUID for the stack.id of all stacks which does not define it"`
		NoGenerate     bool     `help:"Disable code generation for the newly created stacks"`
	} `cmd:"" help:"Creates a stack on the project"`

	Fmt struct {
		Files            []string `arg:"" optional:"true" predictor:"file" help:"files to be formatted"`
		Check            bool     `hidden:"" help:"Lists unformatted files, exit with 0 if all is formatted, 1 otherwise"`
		DetailedExitCode bool     `help:"Return an appropriate exit code (0 = ok, 1 = error, 2 = no error but changes were made)"`
	} `cmd:"" help:"Format all files inside dir recursively"`

	List struct {
		Why                bool   `help:"Shows the reason why the stack has changed"`
		ExperimentalStatus string `hidden:"" help:"Filter by status (Deprecated)"`
		CloudStatus        string `help:"Filter by status. Example: --cloud-status=unhealthy"`
		RunOrder           bool   `default:"false" help:"Sort stacks by order of execution"`
	} `cmd:"" help:"List stacks"`

	Run struct {
		CloudStatus                string `help:"Filter by status. Example: --cloud-status=unhealthy"`
		CloudSyncDeployment        bool   `default:"false" help:"Enable synchronization of stack execution with the Terramate Cloud"`
		CloudSyncDriftStatus       bool   `default:"false" help:"Enable drift detection and synchronization with the Terramate Cloud"`
		CloudSyncPreview           bool   `default:"false" help:"Enable synchronization of review request previews to Terramate Cloud"`
		CloudSyncTerraformPlanFile string `default:"" help:"Enable sync of Terraform plan file"`
		ContinueOnError            bool   `default:"false" help:"Continue executing in other stacks in case of error"`
		NoRecursive                bool   `default:"false" help:"Do not recurse into child stacks"`
		DryRun                     bool   `default:"false" help:"Plan the execution but do not execute it"`
		Reverse                    bool   `default:"false" help:"Reverse the order of execution"`
		Eval                       bool   `default:"false" help:"Evaluate command line arguments as HCL strings"`

		// Note: 0 is not the real default value here, this is just a workaround.
		// Kong doesn't support having 0 as the default value in case the flag isn't set, but K in case it's set without a value.
		// The K case is handled in the custom decoder.
		Parallel kongParallelFlag `short:"j" optional:"true" default:"0" help:"Run independent tasks in parallel"`

		runSafeguardsCliSpec

		Command []string `arg:"" name:"cmd" predictor:"file" passthrough:"" help:"Command to execute"`
	} `cmd:"" help:"Run command in the stacks"`

	Generate struct {
		DetailedExitCode bool `default:"false" help:"Return detailed exit code (0 = ok, 1 = errors, 2 = no errors but changes were made"`
	} `cmd:"" help:"Generate terraform code for stacks"`

	Script struct {
		List struct{} `cmd:"" help:"Show a list of all scripts in the current directory"`
		Tree struct{} `cmd:"" help:"Show a tree of all scripts in the current directory"`
		Info struct {
			Cmds []string `arg:"" optional:"true" passthrough:"" help:"Script to show info"`
		} `cmd:"" help:"Show detailed information about a script"`
		Run struct {
			CloudStatus string `help:"Filter by status. Example: --cloud-status=unhealthy"`
			NoRecursive bool   `default:"false" help:"Do not recurse into child stacks"`
			DryRun      bool   `default:"false" help:"Plan the execution but do not execute it"`

			Cmds []string `arg:"" optional:"true" passthrough:"" help:"Script to execute"`

			// See above comment regarding for run --parallel.
			Parallel kongParallelFlag `short:"j" optional:"true" default:"0" help:"Run independent tasks in parallel"`

			runSafeguardsCliSpec
		} `cmd:"" help:"Run script in stacks"`
	} `cmd:"" help:"Terramate Script commands"`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`

	Debug struct {
		Show struct {
			Metadata        struct{} `cmd:"" help:"Shows metadata available on the project"`
			Globals         struct{} `cmd:"" help:"List globals for all stacks"`
			GenerateOrigins struct {
			} `cmd:"" help:"Show generate debug information"`
			RuntimeEnv struct{} `cmd:"" help:"List run environment variables for all stacks"`
		} `cmd:"" help:"Show information available in the project"`
	} `cmd:"" help:"Terramate debugging commands"`

	Cloud struct {
		Login struct{} `cmd:"" help:"login for cloud.terramate.io"`
		Info  struct{} `cmd:"" help:"cloud information status"`
		Drift struct {
			Show struct {
			} `cmd:"" help:"show drifts"`
		} `cmd:"" help:"manage cloud drifts"`
	} `cmd:"" help:"Terramate Cloud commands"`

	Experimental struct {
		Clone struct {
			SrcDir          string `arg:"" name:"srcdir" predictor:"file" help:"Path of the stack being cloned"`
			DestDir         string `arg:"" name:"destdir" predictor:"file" help:"Path of the new stack"`
			SkipChildStacks bool   `default:"false" help:"Clone ignores child stacks"`
		} `cmd:"" help:"Clones a stack"`

		Trigger struct {
			Stack              string `arg:"" optional:"true" name:"stack" predictor:"file" help:"Path of the stack being triggered"`
			Reason             string `default:"" name:"reason" help:"Reason for the stack being triggered"`
			ExperimentalStatus string `hidden:"" help:"Filter by status (Deprecated)"`
			CloudStatus        string `help:"Filter by status. Example: --cloud-status=unhealthy"`
		} `cmd:"" help:"Triggers a stack"`

		RunGraph struct {
			Outfile string `short:"o" predictor:"file" default:"" help:"Output .dot file"`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\""`
		} `cmd:"" help:"Generate a graph of the execution order"`

		RunOrder struct {
			Basedir string `arg:"" optional:"true" help:"Base directory to search stacks"`
		} `hidden:"" cmd:"" help:"Show the topological ordering of the stacks"`

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
			Login struct{} `cmd:"" help:"login for cloud.terramate.io"`
			Info  struct{} `cmd:"" help:"cloud information status"`
			Drift struct {
				Show struct {
				} `cmd:"" help:"show drifts"`
			} `cmd:"" help:"manage cloud drifts"`
		} `cmd:"" hidden:"" help:"Terramate Cloud commands"`
	} `cmd:"" help:"Experimental features (may change or be removed in the future)"`
}

type runSafeguardsCliSpec struct {
	// Note: The `name` and `short` are being used to define the -X flag without longer version.
	DisableSafeguardsAll bool               `default:"false" name:"disable-safeguards=all" short:"X" help:"Disable all safeguards"`
	DisableSafeguards    safeguard.Keywords `env:"TM_DISABLE_SAFEGUARDS" enum:"git,all,none,git-untracked,git-uncommitted,outdated-code,git-out-of-sync" help:"Disable safeguards: {all,none,git,git-untracked,git-uncommitted,outdated-code,git-out-of-sync}"`

	DeprecatedDisableCheckGenCode   bool `hidden:"" default:"false" name:"disable-check-gen-code" env:"TM_DISABLE_CHECK_GEN_CODE" help:"Disable outdated generated code check"`
	DeprecatedDisableCheckGitRemote bool `hidden:"" default:"false" name:"disable-check-git-remote" env:"TM_DISABLE_CHECK_GIT_REMOTE" help:"Disable checking if local default branch is updated with remote"`
}

type deprecatedGlobalSafeguardsCliSpec struct {
	DeprecatedDisableCheckGitUntracked   bool `hidden:"true" optional:"true" name:"disable-check-git-untracked" default:"false" env:"TM_DISABLE_CHECK_GIT_UNTRACKED" help:"Disable git check for untracked files"`
	DeprecatedDisableCheckGitUncommitted bool `hidden:"true" optional:"true" name:"disable-check-git-uncommitted" default:"false" env:"TM_DISABLE_CHECK_GIT_UNCOMMITTED" help:"Disable git check for uncommitted files"`
}

type safeguards struct {
	DisableCheckGitUntracked          bool
	DisableCheckGitUncommitted        bool
	DisableCheckGitRemote             bool
	DisableCheckGenerateOutdatedCheck bool

	reEnabled bool
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
	version        string
	ctx            *kong.Context
	parsedArgs     *cliSpec
	clicfg         cliconfig.Config
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	output         out.O // Deprecated: use printer.Stdout/Stderr
	exit           bool
	prj            project
	httpClient     http.Client
	cloud          cloudConfig
	uimode         UIMode
	affectedStacks []stack.Entry

	safeguards safeguards

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
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.Exit(func(status int) {
			// Avoid kong aborting entire process since we designed CLI as lib
			kongExit = true
			kongExitStatus = status
		}),
		kong.Writers(stdout, stderr),
	)
	if err != nil {
		fatal("creating cli parser", err)
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
		fatal(sprintf("parsing cli args %v", args), err)
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
		fatal("failed to load cli configuration file", err)
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
			title := sprintf("Please either export the %s environment variable or "+
				"set the homeTerramateDir option in the %s configuration file",
				cliconfig.DirEnv,
				cliconfig.Filename,
			)

			fatal(title, err)
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
			fatal("installing shell completions", err)
		}
		return &cli{exit: true}
	case "experimental cloud login": // Deprecated: use cloud login
		fallthrough
	case "cloud login":
		err := googleLogin(output, idpkey(), clicfg)
		if err != nil {
			fatal("authentication failed", err)
		}
		output.MsgStdOut("authenticated successfully")
		return &cli{exit: true}
	}

	wd, err := os.Getwd()
	if err != nil {
		fatal("getting workdir", err)
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
			fatal(sprintf("changing working dir to %s", parsedArgs.Chdir), err)
		}

		wd, err = os.Getwd()
		if err != nil {
			fatal("getting workdir: %s", err)
		}
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		fatal(sprintf("evaluating symlinks on working dir: %s", wd), err)
	}

	prj, foundRoot, err := lookupProject(wd)
	if err != nil {
		fatal("unable to parse configuration", err)
	}

	if !foundRoot {
		output.MsgStdErr(`Error: Terramate was unable to detect a project root.

Please ensure you run Terramate inside a Git repository or create a new one here by calling 'git init'.

Using Terramate together with Git is the recommended way.

Alternatively you can create a Terramate config to make the current directory the project root.

Please see https://terramate.io/docs/cli/configuration/project-setup for details.
`)
		os.Exit(1)
	}

	err = prj.setDefaults()
	if err != nil {
		fatal("setting configuration", err)
	}

	if parsedArgs.Changed && !prj.isRepo {
		fatal("flag --changed provided but no git repository found", nil)
	}

	if parsedArgs.Changed && !prj.hasCommits() {
		fatal("flag --changed requires a repository with at least two commits", nil)
	}

	uimode := HumanMode
	if val := os.Getenv("CI"); envVarIsSet(val) {
		uimode = AutomationMode
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
		uimode:     uimode,

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
	case "fmt <files>":
		c.format()
	case "create <path>":
		c.createStack()
	case "create":
		c.scanCreate()
	case "list":
		c.setupGit()
		c.printStacks()
	case "run":
		fatal("no command specified", nil)
	case "run <cmd>":
		c.setupGit()
		c.setupSafeguards(c.parsedArgs.Run.runSafeguardsCliSpec)
		c.runOnStacks()
	case "generate":
		c.generate()
	case "experimental clone <srcdir> <destdir>":
		c.cloneStack()
	case "experimental trigger":
		c.triggerStackByFilter()
	case "experimental trigger <stack>":
		c.triggerStack(c.parsedArgs.Experimental.Trigger.Stack)
	case "experimental vendor download <source> <ref>":
		c.vendorDownload()
	case "debug show globals":
		c.setupGit()
		c.printStacksGlobals()
	case "debug show generate-origins":
		c.setupGit()
		c.generateDebug()
	case "debug show metadata":
		c.setupGit()
		c.printMetadata()
	case "experimental run-graph":
		c.setupGit()
		c.generateGraph()
	case "experimental run-order":
		c.setupGit()
		c.printRunOrder(false)
	case "debug show runtime-env":
		c.setupGit()
		c.printRuntimeEnv()
	case "experimental eval":
		fatal("no expression specified", nil)
	case "experimental eval <expr>":
		c.eval()
	case "experimental partial-eval":
		fatal("no expression specified", nil)
	case "experimental partial-eval <expr>":
		c.partialEval()
	case "experimental get-config-value":
		fatal("no variable specified", nil)
	case "experimental get-config-value <var>":
		c.getConfigValue()
	case "experimental cloud info": // Deprecated
		fallthrough
	case "cloud info":
		c.cloudInfo()
	case "experimental cloud drift show": // Deprecated
		fallthrough
	case "cloud drift show":
		c.cloudDriftShow()
	case "script list":
		c.checkScriptEnabled()
		c.printScriptList()
	case "script tree":
		c.checkScriptEnabled()
		c.printScriptTree()
	case "script info":
		c.checkScriptEnabled()
		fatal("no script specified", nil)
	case "script info <cmds>":
		c.checkScriptEnabled()
		c.printScriptInfo()
	case "script run":
		c.checkScriptEnabled()
		fatal("no script specified", nil)
	case "script run <cmds>":
		c.checkScriptEnabled()
		c.setupGit()
		c.setupSafeguards(c.parsedArgs.Script.Run.runSafeguardsCliSpec)
		c.runScript()
	default:
		fatal("unexpected command sequence", nil)
	}
}

func (s *kongParallelFlag) Decode(ctx *kong.DecodeContext) error {
	if ctx.Scan.Peek().Type == kong.FlagValueToken {
		t, err := ctx.Scan.PopValue("counter")
		if err != nil {
			return err
		}

		switch v := t.Value.(type) {
		case string:
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return stdfmt.Errorf("expected a counter but got %q (%T)", t, t.Value)
			}
			s.Value = int(n)

		case int, int8, int16, int32, int64:
			t := reflect.ValueOf(v)
			s.Value = int(t.Int())

		default:
			return stdfmt.Errorf("expected a counter but got %q (%T)", t, t.Value)
		}
		return nil
	}

	s.Value = defaultParallelRunCount

	return nil
}

func (c *cli) setupSafeguards(run runSafeguardsCliSpec) {
	global := c.parsedArgs.deprecatedGlobalSafeguardsCliSpec

	// handle deprecated flags as --disable-safeguards
	if global.DeprecatedDisableCheckGitUncommitted {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-uncommitted")
	}
	if global.DeprecatedDisableCheckGitUntracked {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-untracked")
	}
	if run.DeprecatedDisableCheckGitRemote {
		run.DisableSafeguards = append(run.DisableSafeguards, "git-out-of-sync")
	}
	if run.DeprecatedDisableCheckGenCode {
		run.DisableSafeguards = append(run.DisableSafeguards, "outdated-code")
	}
	if run.DisableSafeguardsAll {
		run.DisableSafeguards = append(run.DisableSafeguards, "all")
	}

	if run.DisableSafeguards.Has(safeguard.All) && run.DisableSafeguards.Has(safeguard.None) {
		fatal("Disabling safeguards",
			errors.E(clitest.ErrSafeguardKeywordValidation,
				`the safeguards keywords "all" and "none" are incompatible`),
		)
	}

	c.safeguards.DisableCheckGitUncommitted = run.DisableSafeguards.Has(safeguard.GitUncommitted, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGitUntracked = run.DisableSafeguards.Has(safeguard.GitUntracked, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGitRemote = run.DisableSafeguards.Has(safeguard.GitOutOfSync, safeguard.All, safeguard.Git)
	c.safeguards.DisableCheckGenerateOutdatedCheck = run.DisableSafeguards.Has(safeguard.Outdated, safeguard.All)
	if run.DisableSafeguards.Has("none") {
		c.safeguards = safeguards{}
		c.safeguards.reEnabled = true
	}
}

func (c *cli) setupGit() {
	if !c.parsedArgs.Changed || !c.prj.isGitFeaturesEnabled() {
		return
	}

	remoteCheckFailed := false

	if err := c.prj.checkDefaultRemote(); err != nil {
		if c.prj.git.remoteConfigured {
			fatal("checking git default remote", err)
		} else {
			remoteCheckFailed = true
		}
	}

	if c.parsedArgs.GitChangeBase != "" {
		c.prj.baseRef = c.parsedArgs.GitChangeBase
	} else if remoteCheckFailed {
		c.prj.baseRef = c.prj.defaultLocalBaseRef()
	} else {
		c.prj.baseRef = c.prj.defaultBaseRef()
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
		fatal(sprintf("parsing module source %s: %s", source, err), nil)
	}
	if parsedSource.Ref != "" {
		fatal(sprintf("module source %s should not contain a reference", source), nil)
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
	if c.parsedArgs.Experimental.Vendor.Download.Dir != "" {

		dir := c.parsedArgs.Experimental.Vendor.Download.Dir
		if !path.IsAbs(dir) {
			dir = prj.PrjAbsPath(c.rootdir(), c.wd()).Join(dir).String()
		}
		return prj.NewPath(dir)
	}

	checkVendorDir := func(dir string) prj.Path {
		if !path.IsAbs(dir) {
			fatal(sprintf("vendorDir %s defined is not an absolute path", dir), nil)
		}
		return prj.NewPath(dir)
	}

	dotTerramate := filepath.Join(c.rootdir(), ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {

		cfg, err := hcl.ParseDir(c.rootdir(), filepath.Join(c.rootdir(), ".terramate"))
		if err != nil {
			fatal("parsing vendor dir configuration on .terramate", err)
		}

		if hasVendorDirConfig(cfg) {

			return checkVendorDir(cfg.Vendor.Dir)
		}
	}

	hclcfg := c.rootNode()
	if hasVendorDirConfig(hclcfg) {

		return checkVendorDir(hclcfg.Vendor.Dir)
	}

	return prj.NewPath(defaultVendorDir)
}

func hasVendorDirConfig(cfg hcl.Config) bool {
	return cfg.Vendor != nil && cfg.Vendor.Dir != ""
}

func (c *cli) triggerStackByFilter() {
	expStatus := c.parsedArgs.Experimental.Trigger.ExperimentalStatus
	cloudStatus := c.parsedArgs.Experimental.Trigger.CloudStatus
	if expStatus != "" && cloudStatus != "" {
		fatal("--experimental-status and --cloud-status cannot be used together", nil)
	}

	statusStr := expStatus
	if cloudStatus != "" {
		statusStr = cloudStatus
	}

	if statusStr == "" {
		fatal("trigger command expects either a stack path or the --cloud-status flag", nil)
	}

	status := parseStatusFilter(statusStr)
	stacksReport, err := c.listStacks(false, status)
	if err != nil {
		fatal("unable to list stacks", err)
	}

	for _, st := range stacksReport.Stacks {
		c.triggerStack(st.Stack.Dir.String())
	}
}

func (c *cli) triggerStack(stack string) {
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
		fatal("symlinks are disallowed in the stack path", nil)
	}

	if !strings.HasPrefix(stack, c.rootdir()) {
		fatal(sprintf("stack %s is outside project", stack), nil)
	}

	stackPath := prj.PrjAbsPath(c.rootdir(), stack)
	if err := trigger.Create(c.cfg(), stackPath, reason); err != nil {
		fatal("unable to create trigger", err)
	}

	c.output.MsgStdOut("Created trigger for stack %q", stackPath)
}

func (c *cli) cloneStack() {
	srcdir := c.parsedArgs.Experimental.Clone.SrcDir
	destdir := c.parsedArgs.Experimental.Clone.DestDir
	skipChildStacks := c.parsedArgs.Experimental.Clone.SkipChildStacks

	// Convert to absolute paths
	absSrcdir := filepath.Join(c.wd(), srcdir)
	absDestdir := filepath.Join(c.wd(), destdir)

	n, err := stack.Clone(c.cfg(), absDestdir, absSrcdir, skipChildStacks)
	if err != nil {
		fatal(sprintf("cloning %s to %s", srcdir, destdir), err)
	}

	c.output.MsgStdOut("Cloned %d stack(s) from %s to %s with success", n, srcdir, destdir)
	c.output.MsgStdOut("Generating code on the new cloned stack(s)")

	c.generate()
}

func (c *cli) generate() {
	report, vendorReport := c.gencodeWithVendor()

	c.output.MsgStdOut(report.Full())

	vendorReport.RemoveIgnoredByKind(download.ErrAlreadyVendored)

	exitCode := 0

	if !vendorReport.IsEmpty() {
		c.output.MsgStdOut(vendorReport.String())

	}

	if c.parsedArgs.Generate.DetailedExitCode {
		if len(report.Successes) > 0 || !vendorReport.IsEmpty() {
			exitCode = 2
		}
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		exitCode = 1
	}

	os.Exit(exitCode)
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
	if !c.prj.isGitFeaturesEnabled() || c.safeguards.DisableCheckGitUntracked {
		return false
	}

	if c.safeguards.reEnabled {
		return !c.safeguards.DisableCheckGitUntracked
	}

	cfg := c.rootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUntracked)
}

func (c *cli) checkGitUncommited() bool {
	if !c.prj.isGitFeaturesEnabled() || c.safeguards.DisableCheckGitUncommitted {
		return false
	}

	if c.safeguards.reEnabled {
		return !c.safeguards.DisableCheckGitUncommitted
	}

	cfg := c.rootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUncommitted)
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
			fatal(msg, nil)
		} else {
			log.Warn().Msg(msg)
		}
	}

	if c.checkGitUncommited() && len(c.prj.git.repoChecks.UncommittedFiles) > 0 {
		const msg = "repository has uncommitted files"
		if shouldAbort {
			fatal(msg, nil)
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

	if !c.gitSafeguardRemoteEnabled() {
		logger.Debug().Msg("Safeguard default-branch-is-reachable is disabled.")
		return
	}

	if err := c.prj.checkRemoteDefaultBranchIsReachable(); err != nil {
		fatal("unable to reach remote default branch", err)
	}
}

func (c *cli) listStacks(isChanged bool, status cloudstack.FilterStatus) (*stack.Report, error) {
	var (
		err    error
		report *stack.Report
	)

	mgr := c.stackManager()

	if isChanged {
		report, err = mgr.ListChanged(c.baseRef())
	} else {
		report, err = mgr.List()
	}

	if report != nil {
		// memoize the list of affected stacks so they can be retrieved later
		// without computing the list again
		c.affectedStacks = report.Stacks
	}

	if status != cloudstack.NoFilter {
		err := c.setupCloudConfig()
		if err != nil {
			return nil, err
		}

		repoURL, err := c.prj.git.wrapper.URL(c.prj.gitcfg().DefaultRemote)
		if err != nil {
			return nil, errors.E("failed to retrieve repository URL but it's needed for filtering stacks", err)
		}

		repository := cloud.NormalizeGitURI(repoURL)
		if repository == "local" {
			return nil, errors.E("%s status filter does not work with filesystem based remotes: %s", status.String(), repoURL)
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
		defer cancel()
		cloudStacks, err := c.cloud.client.StacksByStatus(ctx, c.cloud.run.orgUUID, status)
		if err != nil {
			return nil, err
		}

		cloudStacksMap := map[string]bool{}
		for _, stack := range cloudStacks {
			if stack.Repository == repository {
				cloudStacksMap[stack.MetaID] = true
			}
		}

		localStacks := report.Stacks
		var stacks []stack.Entry

		for _, stack := range localStacks {
			if cloudStacksMap[stack.Stack.ID] {
				stacks = append(stacks, stack)
			}
		}
		report.Stacks = stacks
	}

	if err != nil {
		return nil, err
	}

	c.prj.git.repoChecks = report.Checks
	return report, nil
}

func (c *cli) scanCreate() {
	if c.parsedArgs.Create.EnsureStackIds && c.parsedArgs.Create.AllTerraform {
		fatal("Invalid args", errors.E("--all-terraform conflicts with --ensure-stack-ids"))
	}

	if !c.parsedArgs.Create.AllTerraform && !c.parsedArgs.Create.EnsureStackIds {
		fatal(
			"Invalid args",
			errors.E("terramate create requires a path or --all-terraform or --ensure-stack-ids"),
		)
	}

	var flagname string
	if c.parsedArgs.Create.EnsureStackIds {
		flagname = "--ensure-stack-ids"
	} else {
		flagname = "--all-terraform"
	}

	if c.parsedArgs.Create.ID != "" ||
		c.parsedArgs.Create.Name != "" ||
		c.parsedArgs.Create.Path != "" ||
		c.parsedArgs.Create.Description != "" ||
		c.parsedArgs.Create.IgnoreExisting ||
		len(c.parsedArgs.Create.After) != 0 ||
		len(c.parsedArgs.Create.Before) != 0 ||
		len(c.parsedArgs.Create.Import) != 0 {

		fatal(
			"Invalid args",
			errors.E(
				"%s is incompatible with path and the flags: "+
					"--id,"+
					" --name, "+
					"--description, "+
					"--after, "+
					"--before, "+
					"--import, "+
					" --ignore-existing",
				flagname,
			),
		)
	}

	if c.parsedArgs.Create.AllTerraform {
		c.initTerraform()
		return
	}

	c.ensureStackID()
}

func (c *cli) initTerraform() {
	err := c.initDir(c.wd())
	if err != nil {
		fatal("failed to initialize some directories", err)
	}

	if c.parsedArgs.Create.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return
	}

	root, err := config.LoadRoot(c.rootdir())
	if err != nil {
		fatal("reloading the configuration", err)
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
	pdir := prj.PrjAbsPath(c.rootdir(), baseDir)
	var isStack bool
	tree, found := c.prj.root.Lookup(pdir)
	if found {
		isStack = tree.IsStack()
	}

	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		fatal("unable to read directory while listing directory entries", err)
	}

	errs := errors.L()
	for _, f := range dirs {
		path := filepath.Join(baseDir, f.Name())
		if strings.HasPrefix(f.Name(), ".") {
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
			continue
		}

		found, err := tf.IsStack(path)
		if err != nil {
			fatal("parsing terraform", err)
		}

		if !found {
			continue
		}

		stackDir := baseDir
		stackID, err := uuid.NewRandom()
		dirBasename := filepath.Base(stackDir)
		if err != nil {
			fatal("creating stack UUID", err)
		}
		stackSpec := config.Stack{
			Dir:         prj.PrjAbsPath(c.rootdir(), stackDir),
			ID:          stackID.String(),
			Name:        dirBasename,
			Description: dirBasename,
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
	if c.parsedArgs.Create.AllTerraform || c.parsedArgs.Create.EnsureStackIds {
		c.scanCreate()
		return
	}

	stackHostDir := filepath.Join(c.wd(), c.parsedArgs.Create.Path)

	stackID := c.parsedArgs.Create.ID
	if stackID == "" {

		id, err := uuid.NewRandom()
		if err != nil {
			fatal("creating stack UUID", err)
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

	var tags []string
	for _, tag := range c.parsedArgs.Tags {
		tags = append(tags, strings.Split(tag, ",")...)
	}

	stackSpec := config.Stack{
		Dir:         prj.PrjAbsPath(c.rootdir(), stackHostDir),
		ID:          stackID,
		Name:        stackName,
		Description: stackDescription,
		After:       c.parsedArgs.Create.After,
		Before:      c.parsedArgs.Create.Before,
		Tags:        tags,
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

		fatal("Cannot create stack", err)
	}

	printer.Stdout.Success("Created stack " + stackSpec.Dir.String())

	if c.parsedArgs.Create.NoGenerate {
		log.Debug().Msg("code generation on stack creation disabled")
		return
	}

	err = c.prj.root.LoadSubTree(stackSpec.Dir)
	if err != nil {
		fatal("Unable to load new stack", err)
	}

	report, vendorReport := c.gencodeWithVendor()
	if report.HasFailures() {
		printer.Stdout.ErrorWithDetails("Code generation failed", stdfmt.Errorf(report.Minimal()))
	}

	if vendorReport.HasFailures() {
		printer.Stdout.ErrorWithDetails("Code generation failed", stdfmt.Errorf(vendorReport.String()))
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		os.Exit(1)
	}

	c.output.MsgStdOutV(report.Minimal())
	c.output.MsgStdOutV(vendorReport.String())
}

func (c *cli) format() {
	if c.parsedArgs.Fmt.Check && c.parsedArgs.Fmt.DetailedExitCode {
		fatal("Invalid args", errors.E("--check conflicts with --detailed-exit-code"))
	}

	var results []fmt.FormatResult
	switch len(c.parsedArgs.Fmt.Files) {
	case 0:
		var err error
		results, err = fmt.FormatTree(c.wd())
		if err != nil {
			fatal(sprintf("formatting directory %s", c.wd()), err)
		}
	case 1:
		if c.parsedArgs.Fmt.Files[0] == "-" {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatal("reading stdin", err)
			}
			original := string(content)
			formatted, err := fmt.Format(original, "<stdin>")
			if err != nil {
				fatal("formatting stdin", err)
			}

			if c.parsedArgs.Fmt.Check {
				var status int
				if formatted != original {
					status = 1
				}
				os.Exit(status)
			}

			stdfmt.Print(formatted)
			return
		}

		fallthrough
	default:
		var err error
		results, err = fmt.FormatFiles(c.wd(), c.parsedArgs.Fmt.Files)
		if err != nil {
			fatal("formatting files", err)
		}
	}

	for _, res := range results {
		path := strings.TrimPrefix(res.Path(), c.wd()+string(filepath.Separator))
		c.output.MsgStdOut(path)
	}

	if len(results) > 0 {
		if c.parsedArgs.Fmt.Check {
			os.Exit(1)
		}

		if c.parsedArgs.Fmt.DetailedExitCode {
			os.Exit(2)
		}
	}

	errs := errors.L()
	for _, res := range results {
		errs.Append(res.Save())
	}

	if err := errs.AsError(); err != nil {
		fatal("saving files formatted files", err)
	}
}

func (c *cli) printStacks() {
	if c.parsedArgs.List.Why && !c.parsedArgs.Changed {
		fatal("Invalid args", errors.E("the --why flag must be used together with --changed"))
	}

	expStatus := c.parsedArgs.List.ExperimentalStatus
	cloudStatus := c.parsedArgs.List.CloudStatus
	if expStatus != "" && cloudStatus != "" {
		fatal("Invalid args", errors.E("--experimental-status and --cloud-status cannot be used together"))
	}

	statusStr := expStatus
	if cloudStatus != "" {
		statusStr = cloudStatus
	}

	status := parseStatusFilter(statusStr)
	report, err := c.listStacks(c.parsedArgs.Changed, status)
	if err != nil {
		fatal("Unable to list stacks", err)
	}
	c.gitFileSafeguards(false)

	c.printStacksList(report.Stacks, c.parsedArgs.List.Why, c.parsedArgs.List.RunOrder)
}

func (c *cli) printStacksList(allStacks []stack.Entry, why bool, runOrder bool) {
	filteredStacks := c.filterStacks(allStacks)

	reasons := map[string]string{}
	stacks := make(config.List[*config.SortableStack], len(filteredStacks))
	for i, entry := range filteredStacks {
		stacks[i] = entry.Stack.Sortable()
		reasons[entry.Stack.ID] = entry.Reason
	}

	if runOrder {
		var failReason string
		var err error
		failReason, err = run.Sort(c.cfg(), stacks,
			func(s *config.SortableStack) *config.Stack { return s.Stack })
		if err != nil {
			fatal("Invalid stack configuration", errors.E(err, failReason))
		}
	}

	for _, s := range stacks {
		dir := s.Dir().String()
		friendlyDir, ok := c.friendlyFmtDir(dir)
		if !ok {
			printer.Stderr.Error(stdfmt.Sprintf("Unable to format stack dir %s", dir))
			printer.Stdout.Println(dir)
			continue
		}

		if why {
			printer.Stdout.Println(stdfmt.Sprintf("%s - %s", friendlyDir, reasons[s.ID]))
		} else {
			printer.Stdout.Println(friendlyDir)
		}
	}
}

func parseStatusFilter(strStatus string) cloudstack.FilterStatus {
	status := cloudstack.NoFilter
	if strStatus != "" {
		status = cloudstack.NewStatusFilter(strStatus)
		if status.Is(cloudstack.Unrecognized) {
			fatal(sprintf("unrecognized stack filter: %s", strStatus), nil)
		}
	}
	return status
}

func (c *cli) printRuntimeEnv() {
	report, err := c.listStacks(c.parsedArgs.Changed, cloudstack.NoFilter)
	if err != nil {
		fatal("listing stacks", err)
	}

	for _, stackEntry := range c.filterStacks(report.Stacks) {
		envVars, err := run.LoadEnv(c.cfg(), stackEntry.Stack)
		if err != nil {
			fatal("loading stack run environment", err)
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

	switch c.parsedArgs.Experimental.RunGraph.Label {
	case "stack.name":
		logger.Debug().Msg("Set label to stack name.")

		getLabel = func(s *config.Stack) string { return s.Name }
	case "stack.dir":
		logger.Debug().Msg("Set label stack directory.")

		getLabel = func(s *config.Stack) string { return s.Dir.String() }
	default:
		fatal(`-label expects the values "stack.name" or "stack.dir"`, nil)
	}

	entries, err := stack.List(c.cfg().Tree())
	if err != nil {
		fatal("listing stacks to build graph", err)
	}

	logger.Debug().Msg("Create new graph.")

	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New[*config.Stack]()

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
			fatal("building order tree", err)
		}
	}

	for _, id := range graph.IDs() {
		val, err := graph.Node(id)
		if err != nil {
			fatal("generating graph", err)
		}

		generateDot(dotGraph, graph, id, val, getLabel)
	}

	logger.Debug().
		Msg("Set output of graph.")
	outFile := c.parsedArgs.Experimental.RunGraph.Outfile
	var out io.Writer
	if outFile == "" {

		out = c.stdout
	} else {

		f, err := os.Create(outFile)
		if err != nil {
			fatal(sprintf("opening file %s", outFile), err)
		}

		defer func() {
			if err := f.Close(); err != nil {
				fatal("closing output graph file", err)
			}
		}()

		out = f
	}

	logger.Debug().
		Msg("Write graph to output.")
	_, err = out.Write([]byte(dotGraph.String()))
	if err != nil {
		fatal(sprintf("writing output %s", outFile), err)
	}
}

func generateDot(
	dotGraph *dot.Graph,
	graph *dag.DAG[*config.Stack],
	id dag.ID,
	stackval *config.Stack,
	getLabel func(s *config.Stack) string,
) {
	descendant := dotGraph.Node(getLabel(stackval))
	for _, ancestor := range graph.AncestorsOf(id) {
		s, err := graph.Node(ancestor)
		if err != nil {
			fatal("generating dot file", err)
		}
		ancestorNode := dotGraph.Node(getLabel(s))

		// we invert the graph here.

		edges := dotGraph.FindEdges(ancestorNode, descendant)
		if len(edges) == 0 {
			edge := dotGraph.Edge(ancestorNode, descendant)
			if graph.HasCycle(ancestor) {
				edge.Attr("color", "red")
				continue
			}
		}

		if graph.HasCycle(ancestor) {
			continue
		}

		generateDot(dotGraph, graph, ancestor, s, getLabel)
	}
}

func (c *cli) printRunOrder(friendlyFmt bool) {
	logger := log.With().
		Str("action", "printRunOrder()").
		Str("workingDir", c.wd()).
		Logger()

	stacks, err := c.computeSelectedStacks(false, cloudstack.NoFilter)
	if err != nil {
		fatal("computing selected stacks", err)
	}

	logger.Debug().Msg("Get run order.")
	reason, err := run.Sort(c.cfg(), stacks,
		func(s *config.SortableStack) *config.Stack { return s.Stack })
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			fatal("Invalid stack configuration", errors.E(err, reason))
		} else {
			fatal("Failed to plan execution", err)
		}
	}

	for _, s := range stacks {
		dir := s.Dir().String()
		if !friendlyFmt {
			printer.Stdout.Println(dir)
			continue
		}

		friendlyDir, ok := c.friendlyFmtDir(dir)
		if !ok {
			printer.Stderr.Error(stdfmt.Sprintf("Unable to format stack dir %s", dir))
			printer.Stdout.Println(dir)
			continue
		}
		printer.Stdout.Println(friendlyDir)
	}
}

func (c *cli) generateDebug() {
	// TODO(KATCIPIS): When we introduce config defined on root context
	// we need to know blocks that have root context, since they should
	// not be filtered by stack selection.
	stacks, err := c.computeSelectedStacks(false, cloudstack.NoFilter)
	if err != nil {
		fatal("generate debug: selecting stacks", err)
	}

	selectedStacks := map[prj.Path]struct{}{}
	for _, stack := range stacks {
		log.Debug().Msgf("selected stack: %s", stack.Dir())

		selectedStacks[stack.Dir()] = struct{}{}
	}

	results, err := generate.Load(c.cfg(), c.vendorDir())
	if err != nil {
		fatal("generate debug: loading generated code", err)
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
	report, err := c.listStacks(c.parsedArgs.Changed, cloudstack.NoFilter)
	if err != nil {
		fatal("listing stacks globals: listing stacks", err)
	}

	for _, stackEntry := range c.filterStacks(report.Stacks) {
		stack := stackEntry.Stack
		report := globals.ForStack(c.cfg(), stack)
		if err := report.AsError(); err != nil {
			fatal(sprintf("listing stacks globals: loading stack at %s", stack.Dir), err)
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

	report, err := c.listStacks(c.parsedArgs.Changed, cloudstack.NoFilter)
	if err != nil {
		fatal("loading metadata: listing stacks", err)
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
	if c.safeguards.DisableCheckGenerateOutdatedCheck {
		return false
	}

	if c.safeguards.reEnabled {
		return !c.safeguards.DisableCheckGenerateOutdatedCheck
	}

	cfg := c.rootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.Outdated)

}

func (c *cli) ensureStackID() {
	report, err := c.listStacks(false, cloudstack.NoFilter)
	if err != nil {
		fatal("listing stacks", err)
	}

	for _, entry := range report.Stacks {
		if entry.Stack.ID != "" {
			continue
		}

		id, err := stack.UpdateStackID(entry.Stack.HostDir(c.cfg()))
		if err != nil {
			fatal(sprintf("failed to update stack.id of stack %s", entry.Stack.Dir), err)
		}

		c.output.MsgStdOut("Generated ID %s for stack %s", id, entry.Stack.Dir)
	}
}

func (c *cli) eval() {
	ctx := c.detectEvalContext(c.parsedArgs.Experimental.Eval.Global)
	for _, exprStr := range c.parsedArgs.Experimental.Eval.Exprs {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal("unable to parse expression", err)
		}
		val, err := ctx.Eval(expr)
		if err != nil {
			fatal(sprintf("eval %q", exprStr), err)
		}
		c.outputEvalResult(val, c.parsedArgs.Experimental.Eval.AsJSON)
	}
}

func (c *cli) partialEval() {
	ctx := c.detectEvalContext(c.parsedArgs.Experimental.PartialEval.Global)
	for _, exprStr := range c.parsedArgs.Experimental.PartialEval.Exprs {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal("unable to parse expression", err)
		}
		newexpr, err := ctx.PartialEval(expr)
		if err != nil {
			fatal(sprintf("partial eval %q", exprStr), err)
		}
		c.output.MsgStdOut("%s", string(hclwrite.Format(ast.TokensForExpression(newexpr).Bytes())))
	}
}

func (c *cli) evalRunArgs(st *config.Stack, cmd []string) ([]string, error) {
	ctx := c.setupEvalContext(st, map[string]string{})
	var newargs []string
	for _, arg := range cmd {
		exprStr := `"` + arg + `"`
		expr, err := ast.ParseExpression(exprStr, "<cmd arg>")
		if err != nil {
			return nil, errors.E(err, "parsing %s", exprStr)
		}
		val, err := ctx.Eval(expr)
		if err != nil {
			return nil, errors.E(err, "eval %s", exprStr)
		}
		if !val.Type().Equals(cty.String) {
			return nil, errors.E("cmd line evaluates to type %s but only string is permitted", val.Type().FriendlyName())
		}

		newargs = append(newargs, val.AsString())
	}
	return newargs, nil
}

func (c *cli) getConfigValue() {
	ctx := c.detectEvalContext(c.parsedArgs.Experimental.GetConfigValue.Global)
	for _, exprStr := range c.parsedArgs.Experimental.GetConfigValue.Vars {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal("unable to parse expression", err)
		}

		iteratorTraversal, diags := hhcl.AbsTraversalForExpr(expr)
		if diags.HasErrors() {
			fatal("expected a variable accessor", errors.E(diags))
		}

		varns := iteratorTraversal.RootName()
		if varns != "terramate" && varns != "global" {
			fatal("only terramate and global variables are supported", nil)
		}

		val, err := ctx.Eval(expr)
		if err != nil {
			fatal(sprintf("evaluating expression: %s", exprStr), err)
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
			fatal(sprintf("converting value %s to json", val.GoString()), err)
		}
	} else {
		if val.Type() == cty.String {
			data = []byte(val.AsString())
		} else {
			tokens := ast.TokensForValue(val)
			data = []byte(hclwrite.Format(tokens.Bytes()))
		}
	}

	c.output.MsgStdOut("%s", string(data))
}

func (c *cli) detectEvalContext(overrideGlobals map[string]string) *eval.Context {
	var st *config.Stack
	if config.IsStack(c.cfg(), c.wd()) {
		var err error
		st, err = config.LoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
		if err != nil {
			fatal("setup eval context: loading stack config", err)
		}
	}
	return c.setupEvalContext(st, overrideGlobals)
}

func (c *cli) setupEvalContext(st *config.Stack, overrideGlobals map[string]string) *eval.Context {
	runtime := c.cfg().Runtime()

	var tdir string
	if st != nil {
		tdir = st.HostDir(c.cfg())
		runtime.Merge(st.RuntimeValues(c.cfg()))
	} else {
		tdir = c.wd()
	}

	ctx := eval.NewContext(stdlib.NoFS(tdir))
	ctx.SetNamespace("terramate", runtime)

	wdPath := prj.PrjAbsPath(c.rootdir(), tdir)
	tree, ok := c.cfg().Lookup(wdPath)
	if !ok {
		fatal("Missing configuration", errors.E("configuration at %s not found", wdPath))
	}
	exprs, err := globals.LoadExprs(tree)
	if err != nil {
		fatal("loading globals expressions", err)
	}

	for name, exprStr := range overrideGlobals {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			fatal("unable to parse expression", errors.E(err, "--global %s=%s is an invalid expresssion", name, exprStr))
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
	return val != "" && val != "0" && val != "false"
}

func (c *cli) checkOutdatedGeneratedCode() {
	logger := log.With().
		Str("action", "checkOutdatedGeneratedCode()").
		Logger()

	if !c.checkGenCode() {
		return
	}

	outdatedFiles, err := generate.DetectOutdated(c.cfg(), c.vendorDir())
	if err != nil {
		fatal("failed to check outdated code on project", err)
	}

	for _, outdated := range outdatedFiles {
		logger.Error().
			Str("filename", outdated).
			Msg("outdated code found")
	}

	if len(outdatedFiles) > 0 {
		fatal(errors.E(ErrOutdatedGenCodeDetected).Error(),
			errors.E("please run: 'terramate generate' to update generated code"))
	}
}

func (c *cli) gitSafeguardRemoteEnabled() bool {
	if !c.prj.isGitFeaturesEnabled() || c.safeguards.DisableCheckGitRemote {
		return false
	}

	if c.safeguards.reEnabled {
		return !c.safeguards.DisableCheckGitRemote
	}

	cfg := c.rootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	isDisabled := cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitOutOfSync)
	if isDisabled {
		return false
	}

	if c.prj.git.remoteConfigured {
		return true
	}

	hasRemotes, _ := c.prj.git.wrapper.HasRemotes()
	return hasRemotes
}

func (c *cli) wd() string                   { return c.prj.wd }
func (c *cli) rootdir() string              { return c.prj.rootdir }
func (c *cli) cfg() *config.Root            { return &c.prj.root }
func (c *cli) baseRef() string              { return c.prj.baseRef }
func (c *cli) stackManager() *stack.Manager { return c.prj.stackManager }
func (c *cli) rootNode() hcl.Config         { return c.prj.root.Tree().Node }
func (c *cli) cred() credential             { return c.cloud.client.Credential.(credential) }

func (c *cli) friendlyFmtDir(dir string) (string, bool) {
	return prj.FriendlyFmtDir(c.rootdir(), c.wd(), dir)
}

func (c *cli) computeSelectedStacks(ensureCleanRepo bool, cloudStatus cloudstack.FilterStatus) (config.List[*config.SortableStack], error) {
	report, err := c.listStacks(c.parsedArgs.Changed, cloudStatus)
	if err != nil {
		return nil, err
	}

	c.gitFileSafeguards(ensureCleanRepo)

	entries := c.filterStacks(report.Stacks)
	stacks := make(config.List[*config.SortableStack], len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack.Sortable()
	}

	stacks, err = c.stackManager().AddWantedOf(stacks)
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
		fatal("version check failed", err)
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
		fatal("unable to parse tag clauses", err)
	}
	if found {
		c.tags = clauses
	}

	for _, val := range c.parsedArgs.NoTags {
		err := tag.Validate(val)
		if err != nil {
			fatal("unable validate tag", err)
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

func newGit(basedir string) (*git.Git, error) {
	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
		Env:        os.Environ(),
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

func lookupProject(wd string) (prj project, found bool, err error) {
	prj = project{
		wd: wd,
	}

	rootcfg, rootCfgPath, rootfound, err := config.TryLoadConfig(wd)
	if err != nil {
		return project{}, false, err
	}

	gw, err := newGit(wd)
	if err == nil {
		gitdir, err := gw.Root()
		if err == nil {
			gitabs := gitdir
			if !filepath.IsAbs(gitabs) {
				gitabs = filepath.Join(wd, gitdir)
			}

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

			cfg, err := config.LoadRoot(rootdir)
			if err != nil {
				return project{}, false, err
			}

			gw = gw.With().WorkingDir(rootdir).Wrapper()

			prj.isRepo = true
			prj.root = *cfg
			prj.rootdir = rootdir
			prj.git.wrapper = gw

			mgr := stack.NewGitAwareManager(&prj.root, gw)
			prj.stackManager = mgr

			return prj, true, nil
		}
	}

	if !rootfound {
		return project{}, false, nil
	}

	prj.rootdir = rootCfgPath
	prj.root = *rootcfg
	prj.stackManager = stack.NewManager(&prj.root)
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
		fatal(sprintf("unknown log destination %q", logdest), nil)
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

func fatal(title string, err error) {
	printer.Stderr.Fatal(title, err)
}

// sprintf is an alias for fmt.Sprintf
func sprintf(format string, a ...interface{}) string {
	return stdfmt.Sprintf(format, a...)
}

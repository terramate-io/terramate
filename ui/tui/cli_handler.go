// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate/commands"
	clonecmd "github.com/terramate-io/terramate/commands/clone"
	clouddriftshowcmd "github.com/terramate-io/terramate/commands/cloud/drift/show"
	cloudinfocmd "github.com/terramate-io/terramate/commands/cloud/info"
	logincmd "github.com/terramate-io/terramate/commands/cloud/login"
	compcmd "github.com/terramate-io/terramate/commands/completions"
	generateoriginscmd "github.com/terramate-io/terramate/commands/debug/show/generate_origins"
	debugglobalscmd "github.com/terramate-io/terramate/commands/debug/show/globals"
	debugshowmetadatacmd "github.com/terramate-io/terramate/commands/debug/show/metadata"
	debugshowruntimeenv "github.com/terramate-io/terramate/commands/debug/show/runtime_env"
	evalcmd "github.com/terramate-io/terramate/commands/experimental/eval"
	rungraphcmd "github.com/terramate-io/terramate/commands/experimental/rungraph"
	vendordownloadcmd "github.com/terramate-io/terramate/commands/experimental/vendordownload"
	fmtcmd "github.com/terramate-io/terramate/commands/fmt"
	gencmd "github.com/terramate-io/terramate/commands/generate"
	reqvercmd "github.com/terramate-io/terramate/commands/requiredversion"
	runcmd "github.com/terramate-io/terramate/commands/run"
	scriptinfocmd "github.com/terramate-io/terramate/commands/script/info"
	scriptlistcmd "github.com/terramate-io/terramate/commands/script/list"
	scriptruncmd "github.com/terramate-io/terramate/commands/script/run"
	scripttreecmd "github.com/terramate-io/terramate/commands/script/tree"
	createcmd "github.com/terramate-io/terramate/commands/stack/create"
	listcmd "github.com/terramate-io/terramate/commands/stack/list"
	triggercmd "github.com/terramate-io/terramate/commands/trigger"
	"github.com/terramate-io/terramate/commands/version"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/safeguard"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
	"github.com/terramate-io/terramate/ui/tui/clitest"
	"github.com/terramate-io/terramate/ui/tui/out"

	"github.com/alecthomas/kong"

	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
)

// ErrSetup is the error returned when the CLI fails to setup its initial values.
const ErrSetup errors.Kind = "failed to setup Terramate"

type handlerState struct {
	tags filter.TagClause
}

func handleRootVersionFlagAlone(parsedSpec any, _ *CLI) (name string, val any, run func(c *CLI, value any) error, isset bool) {
	p := parsedSpec.(*FlagSpec)
	if p.VersionFlag {
		return "--version", p.VersionFlag, func(c *CLI, _ any) error {
			fmt.Println(c.version)
			return nil
		}, true
	}
	return "", nil, nil, false
}

// DefaultRootFlagHandlers returns the CLI default flag handlers for global flags
// that can be used alone (without a command).
// For example: terramate --version
func DefaultRootFlagHandlers() []RootFlagHandlers {
	return []RootFlagHandlers{
		handleRootVersionFlagAlone, // handles: terramate --version
	}
}

// DefaultBeforeConfigHandler implements the default flags handling for when
// the config is not yet parsed.
// Use [WithSpecHandler] if you need a different behavior.
func DefaultBeforeConfigHandler(ctx context.Context, c *CLI) (cmd commands.Executor, found bool, cont bool, err error) {
	kctx := ctx.Value(KongContext).(*kong.Context)

	parsedArgs := c.input.(*FlagSpec)
	// profiler is only started if Terramate is built with -tags profiler
	startProfiler(parsedArgs.CPUProfiling)

	err = ConfigureLogging(parsedArgs.LogLevel, parsedArgs.LogFmt,
		parsedArgs.LogDestination, c.state.stdout, c.state.stderr)
	if err != nil {
		return nil, false, false, err
	}

	c.state.verbose = parsedArgs.Verbose

	if parsedArgs.Quiet {
		c.state.verbose = -1
	}

	c.state.output = out.New(c.state.verbose, c.state.stdout, c.state.stderr)

	c.clicfg, err = cliconfig.Load()
	if err != nil {
		printer.Stderr.ErrorWithDetails("failed to load cli configuration file", err)
		return nil, false, false, errors.E(ErrSetup)
	}

	migrateFlagAliases(parsedArgs)

	// cmdline flags override configuration file.

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
			return nil, false, false, errors.E(ErrSetup)

		}
		c.clicfg.UserTerramateDir = homeTmDir
	}

	c.checkpointResponse = make(chan *checkpoint.CheckResponse, 1)
	go runCheckpoint(
		c.version,
		c.clicfg,
		c.checkpointResponse,
	)

	command := kctx.Command()

	switch command {
	case "version":
		return &version.Spec{
			Version:  c.version,
			InfoChan: c.checkpointResponse,
		}, true, false, nil
	case "install-completions":
		return &compcmd.Spec{}, true, false, nil
	case "experimental cloud login": // Deprecated: use cloud login
		fallthrough
	case "cloud login":
		if parsedArgs.Cloud.Login.Github {
			return &logincmd.GithubSpec{
				Printers:  c.printers,
				CliCfg:    c.clicfg,
				Verbosity: parsedArgs.Verbose,
			}, true, false, nil
		} else if !parsedArgs.Cloud.Login.SSO {
			return &logincmd.GoogleSpec{
				Printers:  c.printers,
				CliCfg:    c.clicfg,
				Verbosity: parsedArgs.Verbose,
			}, true, false, nil
		}

		// --sso is handled later
	}

	logger := log.With().
		Str("workingDir", c.state.wd).
		Logger()

	if parsedArgs.Chdir != "" {
		logger.Debug().
			Str("dir", parsedArgs.Chdir).
			Msg("Changing working directory")

		err = os.Chdir(parsedArgs.Chdir)
		if err != nil {
			return nil, false, false, errors.E(ErrSetup, err, "changing working dir to %s", parsedArgs.Chdir)
		}

		c.state.wd, err = os.Getwd()
		if err != nil {
			return nil, false, false, errors.E(ErrSetup, err, "getting workdir")
		}
	}

	c.state.wd, err = filepath.EvalSymlinks(c.state.wd)
	if err != nil {
		return nil, false, false, errors.E(ErrSetup, err, "evaluating symlinks on working dir: %s", c.state.wd)
	}

	if val := os.Getenv("CI"); envVarIsSet(val) {
		c.state.uimode = engine.AutomationMode
	}

	c.state.changeDetectionEnabled = parsedArgs.Changed
	return nil, false, true, nil
}

// DefaultAfterConfigHandler implements the default flags handling for when
// the config is already parsed.
// Use [WithSpecHandler] if you need a different behavior.
func DefaultAfterConfigHandler(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error) {
	kctx := ctx.Value(KongContext).(*kong.Context)
	cmd := kctx.Command()

	logger := log.With().
		Str("action", "DefaultAfterConfigHandler()").
		Str("cmd", cmd).
		Str("workingDir", c.state.wd).
		Logger()

	rv := reqvercmd.Spec{
		Version: c.version,
		Root:    c.state.engine.Config(),
	}

	err := rv.Exec(context.TODO())
	if err != nil {
		return nil, false, false, err
	}

	parsedArgs := c.input.(*FlagSpec)

	if parsedArgs.Changed && !c.Engine().Project().HasCommits() {
		return nil, false, false, errors.E("flag --changed requires a repository with at least two commits")
	}

	var state handlerState
	filters, err := filter.ParseTags(parsedArgs.Tags, parsedArgs.NoTags)
	if err != nil {
		return nil, false, false, err
	}

	state.tags = filters

	logger.Debug().Msg("Handle command.")

	defer stopProfiler(parsedArgs.CPUProfiling)

	switch cmd {
	case "fmt", "fmt <files>":
		c.InitAnalytics("fmt",
			tel.BoolFlag("detailed-exit-code", parsedArgs.Fmt.DetailedExitCode),
		)
		return &fmtcmd.Spec{
			WorkingDir:       c.state.wd,
			Check:            parsedArgs.Fmt.Check,
			DetailedExitCode: parsedArgs.Fmt.DetailedExitCode,
			Files:            parsedArgs.Fmt.Files,
			Printers:         c.printers,
		}, true, false, nil
	case "create <path>":
		c.InitAnalytics("create")
		return &createcmd.Spec{
			Engine:           c.state.engine,
			WorkingDir:       c.state.wd,
			Path:             parsedArgs.Create.Path,
			IgnoreExisting:   parsedArgs.Create.IgnoreExisting,
			AllTerraform:     parsedArgs.Create.AllTerraform,
			AllTerragrunt:    parsedArgs.Create.AllTerragrunt,
			EnsureStackIDs:   parsedArgs.Create.EnsureStackIDs,
			NoGenerate:       parsedArgs.Create.NoGenerate,
			Imports:          parsedArgs.Create.Import,
			StackID:          parsedArgs.Create.ID,
			StackName:        parsedArgs.Create.Name,
			StackDescription: parsedArgs.Create.Description,
			StackTags:        parsedArgs.Tags,
			StackAfter:       parsedArgs.Create.After,
			StackBefore:      parsedArgs.Create.Before,
			StackWatch:       parsedArgs.Create.Watch,
			StackWants:       parsedArgs.Create.Wants,
			StackWantedBy:    parsedArgs.Create.WantedBy,
			Printers:         c.printers,
			Verbosity:        parsedArgs.Verbose,
		}, true, false, nil
	case "create":
		c.InitAnalytics("create",
			tel.BoolFlag("all-terragrunt", parsedArgs.Create.AllTerragrunt),
			tel.BoolFlag("all-terraform", parsedArgs.Create.AllTerraform),
		)
		return &createcmd.Spec{
			Engine:           c.state.engine,
			WorkingDir:       c.state.wd,
			IgnoreExisting:   parsedArgs.Create.IgnoreExisting,
			AllTerraform:     parsedArgs.Create.AllTerraform,
			AllTerragrunt:    parsedArgs.Create.AllTerragrunt,
			EnsureStackIDs:   parsedArgs.Create.EnsureStackIDs,
			NoGenerate:       parsedArgs.Create.NoGenerate,
			Imports:          parsedArgs.Create.Import,
			StackID:          parsedArgs.Create.ID,
			StackName:        parsedArgs.Create.Name,
			StackDescription: parsedArgs.Create.Description,
			StackTags:        parsedArgs.Tags,
			StackAfter:       parsedArgs.Create.After,
			StackBefore:      parsedArgs.Create.Before,
			StackWatch:       parsedArgs.Create.Watch,
			StackWants:       parsedArgs.Create.Wants,
			StackWantedBy:    parsedArgs.Create.WantedBy,
			Printers:         c.printers,
			Verbosity:        parsedArgs.Verbose,
		}, true, false, nil

	case "list":
		c.InitAnalytics("list",
			tel.BoolFlag("filter-changed", parsedArgs.Changed),
			tel.BoolFlag("filter-tags", len(parsedArgs.Tags) != 0),
			tel.StringFlag("filter-status", parsedArgs.List.Status),
			tel.StringFlag("filter-drift-status", parsedArgs.List.DriftStatus),
			tel.StringFlag("filter-deployment-status", parsedArgs.List.DeploymentStatus),
			tel.StringFlag("filter-target", parsedArgs.List.Target),
			tel.BoolFlag("run-order", parsedArgs.List.RunOrder),
		)
		expStatus := parsedArgs.List.ExperimentalStatus
		cloudStatus := parsedArgs.List.Status
		if expStatus != "" && cloudStatus != "" {
			return nil, false, false, errors.E("--experimental-status and --status cannot be used together")
		}

		var statusStr string
		if cloudStatus != "" {
			statusStr = cloudStatus
		} else if expStatus != "" {
			statusStr = expStatus
		}

		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			parsedArgs.List.EnableChangeDetection,
			parsedArgs.List.DisableChangeDetection,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &listcmd.Spec{
			Engine:    c.state.engine,
			GitFilter: gitfilter,
			Target:    parsedArgs.List.Target,
			StatusFilters: listcmd.StatusFilters{
				StackStatus:      statusStr,
				DeploymentStatus: parsedArgs.List.DeploymentStatus,
				DriftStatus:      parsedArgs.List.DriftStatus,
			},
			RunOrder: parsedArgs.List.RunOrder,
			Tags:     parsedArgs.Tags,
			NoTags:   parsedArgs.NoTags,
		}, true, false, nil

	case "generate":
		c.InitAnalytics("generate",
			tel.BoolFlag("detailed-exit-code", parsedArgs.Generate.DetailedExitCode),
			tel.BoolFlag("parallel", parsedArgs.Generate.Parallel > 0),
		)
		return &gencmd.Spec{
			Engine:           c.state.engine,
			WorkingDir:       c.state.wd,
			DetailedExitCode: parsedArgs.Generate.DetailedExitCode,
			Parallel:         parsedArgs.Generate.Parallel,
			PrintReport:      true,
			Printers:         c.printers,
		}, true, false, nil
	case "experimental clone <srcdir> <destdir>":
		c.InitAnalytics("clone")
		return &clonecmd.Spec{
			Engine:          c.state.engine,
			WorkingDir:      c.state.wd,
			SrcDir:          parsedArgs.Experimental.Clone.SrcDir,
			DstDir:          parsedArgs.Experimental.Clone.DestDir,
			SkipChildStacks: parsedArgs.Experimental.Clone.SkipChildStacks,
			NoGenerate:      parsedArgs.Experimental.Clone.NoGenerate,
			Printers:        c.printers,
		}, true, false, nil
	case "experimental vendor download <source> <ref>":
		c.InitAnalytics("vendor-download")
		return &vendordownloadcmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.state.engine,
			Printers:   c.printers,
			Source:     parsedArgs.Experimental.Vendor.Download.Source,
			Reference:  parsedArgs.Experimental.Vendor.Download.Reference,
			Dir:        parsedArgs.Experimental.Vendor.Download.Dir,
		}, true, false, nil
	case "run":
		return nil, false, false, errors.E("no command specified")
	case "run <cmd>":
		c.InitAnalytics("run",
			tel.BoolFlag("filter-changed", parsedArgs.Changed),
			tel.BoolFlag("filter-tags", len(parsedArgs.Tags) != 0),
			tel.StringFlag("filter-status", parsedArgs.Run.Status),
			tel.StringFlag("filter-drift-status", parsedArgs.Run.DriftStatus),
			tel.StringFlag("filter-deployment-status", parsedArgs.Run.DeploymentStatus),
			tel.StringFlag("target", parsedArgs.Run.Target),
			tel.BoolFlag("sync-deployment", parsedArgs.Run.SyncDeployment),
			tel.BoolFlag("sync-drift", parsedArgs.Run.SyncDriftStatus),
			tel.BoolFlag("sync-preview", parsedArgs.Run.SyncPreview),
			tel.StringFlag("terraform-planfile", parsedArgs.Run.TerraformPlanFile),
			tel.StringFlag("tofu-planfile", parsedArgs.Run.TofuPlanFile),
			tel.StringFlag("layer", string(parsedArgs.Run.Layer)),
			tel.BoolFlag("terragrunt", parsedArgs.Run.Terragrunt),
			tel.BoolFlag("reverse", parsedArgs.Run.Reverse),
			tel.BoolFlag("parallel", parsedArgs.Run.Parallel > 0),
			tel.BoolFlag("output-sharing", parsedArgs.Run.EnableSharing),
			tel.BoolFlag("output-mocks", parsedArgs.Run.MockOnFail),
		)
		sf, err := setupSafeguards(parsedArgs, parsedArgs.Run.runSafeguardsCliSpec)
		if err != nil {
			return nil, false, false, err
		}
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			parsedArgs.Run.EnableChangeDetection,
			parsedArgs.Run.DisableChangeDetection,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &runcmd.Spec{
			Engine:     c.Engine(),
			WorkingDir: c.state.wd,
			Safeguards: sf,
			Printers:   c.printers,
			Stdout:     c.state.stdout,
			Stderr:     c.state.stderr,
			Stdin:      c.state.stdin,

			GitFilter:       gitfilter,
			Command:         parsedArgs.Run.Command,
			Quiet:           parsedArgs.Quiet,
			DryRun:          parsedArgs.Run.DryRun,
			Reverse:         parsedArgs.Run.Reverse,
			ScriptRun:       false,
			ContinueOnError: parsedArgs.Run.ContinueOnError,
			Parallel:        parsedArgs.Run.Parallel,
			NoRecursive:     parsedArgs.Run.NoRecursive,
			SyncDeployment:  parsedArgs.Run.SyncDeployment,
			SyncPreview:     parsedArgs.Run.SyncPreview,
			SyncDriftStatus: parsedArgs.Run.SyncDriftStatus,
			StatusFilters: runcmd.StatusFilters{
				StackStatus:      parsedArgs.Run.Status,
				DriftStatus:      parsedArgs.Run.DriftStatus,
				DeploymentStatus: parsedArgs.Run.DeploymentStatus,
			},
			DebugPreviewURL:   parsedArgs.Run.DebugPreviewURL,
			TechnologyLayer:   parsedArgs.Run.Layer,
			TerraformPlanFile: parsedArgs.Run.TerraformPlanFile,
			TofuPlanFile:      parsedArgs.Run.TofuPlanFile,
			Terragrunt:        parsedArgs.Run.Terragrunt,
			EnableSharing:     parsedArgs.Run.EnableSharing,
			MockOnFail:        parsedArgs.Run.MockOnFail,
			EvalCmd:           parsedArgs.Run.Eval,
			Target:            parsedArgs.Run.Target,
			FromTarget:        parsedArgs.Run.FromTarget,
			Tags:              parsedArgs.Tags,
			NoTags:            parsedArgs.NoTags,
			OutputsSharingOptions: engine.OutputsSharingOptions{
				IncludeOutputDependencies: parsedArgs.Run.IncludeOutputDependencies,
				OnlyOutputDependencies:    parsedArgs.Run.OnlyOutputDependencies,
			},
		}, true, false, nil

	case "cloud login":
		if !parsedArgs.Cloud.Login.SSO {
			panic(errors.E(errors.ErrInternal, "please report this as a bug"))
		}
		orgName := c.cloudOrgName()
		if orgName == "" {
			return nil, false, false, errors.E(
				errors.E("No Terramate Cloud organization configured."),
				"Set `terramate.config.cloud.organization` or export `TM_CLOUD_ORGANIZATION` to the organization shortname that you intend to login.",
			)
		}
		return &logincmd.SSOSpec{
			Printers:  c.printers,
			Verbosity: parsedArgs.Verbose,
			OrgName:   orgName,
		}, true, false, nil

	case "cloud info":
		c.InitAnalytics("cloud-info")
		return &cloudinfocmd.Spec{
			Engine:    c.Engine(),
			Printers:  c.printers,
			Verbosity: parsedArgs.Verbose,
		}, true, false, nil
	case "cloud drift show":
		c.InitAnalytics("cloud-drift-show")
		return &clouddriftshowcmd.Spec{
			WorkingDir:  c.state.wd,
			Engine:      c.Engine(),
			Printers:    c.printers,
			Verbosiness: parsedArgs.Verbose,
			Target:      parsedArgs.Cloud.Drift.Show.Target,
		}, true, false, nil

	case "experimental eval":
		return nil, false, false, errors.E("no expression specified")
	case "experimental eval <expr>":
		return &evalcmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			Exprs:      parsedArgs.Experimental.Eval.Exprs,
			Globals:    parsedArgs.Experimental.Eval.Global,
			AsJSON:     parsedArgs.Experimental.Eval.AsJSON,
		}, true, false, nil
	case "experimental partial-eval":
		return nil, false, false, errors.E("no expression specified")
	case "experimental partial-eval <expr>":
		return &evalcmd.PartialSpec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			Exprs:      parsedArgs.Experimental.PartialEval.Exprs,
			Globals:    parsedArgs.Experimental.PartialEval.Global,
		}, true, false, nil
	case "experimental get-config-value":
		return nil, false, false, errors.E("no variable specified")
	case "experimental get-config-value <var>":
		return &evalcmd.GetConfigValueSpec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			Vars:       parsedArgs.Experimental.GetConfigValue.Vars,
			Globals:    parsedArgs.Experimental.GetConfigValue.Global,
			AsJSON:     parsedArgs.Experimental.GetConfigValue.AsJSON,
		}, true, false, nil
	case "experimental trigger": // Deprecated
		parsedArgs.Trigger = parsedArgs.Experimental.Trigger
		fallthrough
	case "trigger":
		c.InitAnalytics("trigger")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil,
			nil,
		)
		if err != nil {
			return nil, false, false, err
		}
		expStatus := parsedArgs.Trigger.ExperimentalStatus
		cloudStatus := parsedArgs.Trigger.Status
		if expStatus != "" && cloudStatus != "" {
			return nil, false, false, errors.E("--experimental-status and --status cannot be used together")
		}

		var statusStr string
		if cloudStatus != "" {
			statusStr = cloudStatus
		} else if expStatus != "" {
			statusStr = expStatus
		}
		if statusStr == "" {
			return nil, false, false, errors.E("trigger command expects either a stack path or the --status flag")
		}
		if statusStr != "" && parsedArgs.Trigger.Recursive {
			return nil, false, false, errors.E("cloud filters such as --status are incompatible with --recursive flag")
		}
		return &triggercmd.FilterSpec{
			Engine:     c.Engine(),
			WorkingDir: c.state.wd,
			Printers:   c.printers,
			GitFilter:  gitfilter,
			StatusFilters: triggercmd.StatusFilters{
				StackStatus: statusStr,

				// TODO(i4k): This is a bug in CLI.
				// Uncomment lines below to fix this in a separate PR.

				// DeploymentStatus: parsedArgs.Trigger.DeploymentStatus,
				// DriftStatus:      parsedArgs.Trigger.DriftStatus,
			},
			Change:       parsedArgs.Trigger.Change,
			IgnoreChange: parsedArgs.Trigger.IgnoreChange,
			Reason:       parsedArgs.Trigger.Reason,
		}, true, false, nil

	case "experimental trigger <stack>": // Deprecated
		parsedArgs.Trigger = parsedArgs.Experimental.Trigger
		fallthrough
	case "trigger <stack>":
		c.InitAnalytics("trigger",
			tel.StringFlag("stack", parsedArgs.Trigger.Stack),
			tel.BoolFlag("change", parsedArgs.Trigger.Change),
			tel.BoolFlag("ignore-change", parsedArgs.Trigger.IgnoreChange),
		)
		if parsedArgs.Trigger.Status != "" && parsedArgs.Trigger.Recursive {
			return nil, false, false, errors.E("cloud filters such as --status are incompatible with --recursive flag")
		}
		return &triggercmd.PathSpec{
			Engine:       c.Engine(),
			WorkingDir:   c.state.wd,
			Printers:     c.printers,
			Change:       parsedArgs.Trigger.Change,
			IgnoreChange: parsedArgs.Trigger.IgnoreChange,
			Reason:       parsedArgs.Trigger.Reason,
			Path:         parsedArgs.Trigger.Stack,
			Recursive:    parsedArgs.Trigger.Recursive,
		}, true, false, nil

	case "script list":
		checkScriptEnabled(c.Config())
		c.InitAnalytics("script-list")
		return &scriptlistcmd.Spec{
			Engine:     c.Engine(),
			Printers:   c.printers,
			WorkingDir: c.state.wd,
		}, true, false, nil
	case "script tree":
		checkScriptEnabled(c.Config())
		c.InitAnalytics("script-tree")
		return &scripttreecmd.Spec{
			Engine:     c.Engine(),
			WorkingDir: c.state.wd,
			Printers:   c.printers,
		}, true, false, nil
	case "script info":
		checkScriptEnabled(c.Config())
		return nil, false, false, errors.E("no script specified")
	case "script info <cmds>":
		checkScriptEnabled(c.Config())
		c.InitAnalytics("script-info")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil, nil,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &scriptinfocmd.Spec{
			Engine:     c.Engine(),
			WorkingDir: c.state.wd,
			Printers:   c.printers,
			GitFilter:  gitfilter,
			Labels:     parsedArgs.Script.Info.Cmds,
		}, true, false, nil
	case "script run":
		checkScriptEnabled(c.Config())
		return nil, false, false, errors.E("no script specified")
	case "script run <cmds>":
		checkScriptEnabled(c.Config())
		c.InitAnalytics("script-run",
			tel.BoolFlag("filter-changed", parsedArgs.Changed),
			tel.BoolFlag("filter-tags", len(parsedArgs.Tags) != 0),
			tel.StringFlag("filter-status", parsedArgs.Script.Run.Status),
			tel.StringFlag("filter-drift-status", parsedArgs.Script.Run.DriftStatus),
			tel.StringFlag("filter-deployment-status", parsedArgs.Script.Run.DeploymentStatus),
			tel.StringFlag("target", parsedArgs.Script.Run.Target),
			tel.BoolFlag("reverse", parsedArgs.Script.Run.Reverse),
			tel.BoolFlag("parallel", parsedArgs.Script.Run.Parallel > 0),
		)
		sf, err := setupSafeguards(parsedArgs, parsedArgs.Script.Run.runSafeguardsCliSpec)
		if err != nil {
			return nil, false, false, err
		}
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			parsedArgs.Script.Run.EnableChangeDetection,
			parsedArgs.Script.Run.DisableChangeDetection,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &scriptruncmd.Spec{
			Engine:     c.Engine(),
			WorkingDir: c.state.wd,
			Safeguards: sf,
			Printers:   c.printers,
			Stdout:     c.state.stdout,
			Stderr:     c.state.stderr,
			Stdin:      c.state.stdin,
			GitFilter:  gitfilter,

			Quiet:           parsedArgs.Quiet,
			DryRun:          parsedArgs.Script.Run.DryRun,
			Reverse:         parsedArgs.Script.Run.Reverse,
			ContinueOnError: parsedArgs.Script.Run.ContinueOnError,
			Parallel:        parsedArgs.Script.Run.Parallel,
			NoRecursive:     parsedArgs.Script.Run.NoRecursive,
			Target:          parsedArgs.Script.Run.Target,
			FromTarget:      parsedArgs.Script.Run.FromTarget,
			Tags:            parsedArgs.Tags,
			NoTags:          parsedArgs.NoTags,
			OutputsSharingOptions: engine.OutputsSharingOptions{
				IncludeOutputDependencies: parsedArgs.Script.Run.IncludeOutputDependencies,
				OnlyOutputDependencies:    parsedArgs.Script.Run.OnlyOutputDependencies,
			},
			StatusFilters: runcmd.StatusFilters{
				StackStatus:      parsedArgs.Script.Run.Status,
				DriftStatus:      parsedArgs.Script.Run.DriftStatus,
				DeploymentStatus: parsedArgs.Script.Run.DeploymentStatus,
			},
			Labels: parsedArgs.Script.Run.Cmds,
		}, true, false, nil
	case "debug show globals":
		c.InitAnalytics("debug-show-globals")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil, nil,
		)
		if err != nil {
			return nil, false, false, err
		}

		return &debugglobalscmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			GitFilter:  gitfilter,
		}, true, false, nil

	case "debug show generate-origins":
		c.InitAnalytics("debug-show-generate-origins")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil, nil,
		)
		if err != nil {
			return nil, false, false, err
		}

		return &generateoriginscmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			GitFilter:  gitfilter,
		}, true, false, nil

	case "debug show metadata":
		c.InitAnalytics("debug-show-metadata")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil, nil,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &debugshowmetadatacmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			GitFilter:  gitfilter,
		}, true, false, nil
	case "debug show runtime-env":
		c.InitAnalytics("debug-show-runtime-env")
		gitfilter, err := engine.NewGitFilter(
			parsedArgs.Changed,
			parsedArgs.GitChangeBase,
			nil, nil,
		)
		if err != nil {
			return nil, false, false, err
		}
		return &debugshowruntimeenv.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			GitFilter:  gitfilter,
		}, true, false, nil

	case "experimental run-graph":
		c.InitAnalytics("graph")
		return &rungraphcmd.Spec{
			WorkingDir: c.state.wd,
			Engine:     c.Engine(),
			Printers:   c.printers,
			Label:      parsedArgs.Experimental.RunGraph.Label,
			OutputFile: parsedArgs.Experimental.RunGraph.Outfile,
		}, true, false, nil
	default:
		return nil, false, false, errors.E("unexpected command sequence")
	}
}

func checkScriptEnabled(cfg *config.Root) {
	if cfg.HasExperiment("scripts") {
		return
	}

	printer.Stderr.Error(`The "scripts" feature is not enabled`)
	printer.Stderr.Println(`In order to enable it you must set the terramate.config.experiments attribute.`)
	printer.Stderr.Println(`Example:

terramate {
  config {
    experiments = ["scripts"]
  }
}`)
	os.Exit(1)
}

func envVarIsSet(val string) bool {
	return val != "" && val != "0" && val != "false"
}

func setupSafeguards(parsedArgs *FlagSpec, runflags runSafeguardsCliSpec) (sf runcmd.Safeguards, err error) {
	global := parsedArgs.deprecatedGlobalSafeguardsCliSpec

	// handle deprecated flags as --disable-safeguards
	if global.DeprecatedDisableCheckGitUncommitted {
		runflags.DisableSafeguards = append(runflags.DisableSafeguards, "git-uncommitted")
	}
	if global.DeprecatedDisableCheckGitUntracked {
		runflags.DisableSafeguards = append(runflags.DisableSafeguards, "git-untracked")
	}
	if runflags.DeprecatedDisableCheckGitRemote {
		runflags.DisableSafeguards = append(runflags.DisableSafeguards, "git-out-of-sync")
	}
	if runflags.DeprecatedDisableCheckGenCode {
		runflags.DisableSafeguards = append(runflags.DisableSafeguards, "outdated-code")
	}
	if runflags.DisableSafeguardsAll {
		runflags.DisableSafeguards = append(runflags.DisableSafeguards, "all")
	}

	if runflags.DisableSafeguards.Has(safeguard.All) && runflags.DisableSafeguards.Has(safeguard.None) {
		return runcmd.Safeguards{}, errors.E(
			errors.E(clitest.ErrSafeguardKeywordValidation,
				`the safeguards keywords "all" and "none" are incompatible`),
			"Disabling safeguards",
		)
	}

	sf.DisableCheckGitUncommitted = runflags.DisableSafeguards.Has(safeguard.GitUncommitted, safeguard.All, safeguard.Git)
	sf.DisableCheckGitUntracked = runflags.DisableSafeguards.Has(safeguard.GitUntracked, safeguard.All, safeguard.Git)
	sf.DisableCheckGitRemote = runflags.DisableSafeguards.Has(safeguard.GitOutOfSync, safeguard.All, safeguard.Git)
	sf.DisableCheckGenerateOutdatedCheck = runflags.DisableSafeguards.Has(safeguard.Outdated, safeguard.All)
	if runflags.DisableSafeguards.Has("none") {
		sf = runcmd.Safeguards{}
		sf.ReEnabled = true
	}
	return sf, nil
}

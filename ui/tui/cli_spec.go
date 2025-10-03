// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/safeguard"
	"github.com/willabides/kongplete"
)

// FlagSpec defines the default Terramate flags and commands.
type FlagSpec struct {
	globalCliFlags `envprefix:"TM_ARG_"`

	deprecatedGlobalSafeguardsCliSpec

	DisableCheckpoint          bool `hidden:"true" optional:"true" default:"false" help:"Disable checkpoint checks for updates."`
	DisableCheckpointSignature bool `hidden:"true" optional:"true" default:"false" help:"Disable checkpoint signature."`
	CPUProfiling               bool `hidden:"true" optional:"true" default:"false" help:"Create a CPU profile file when running"`

	Create struct {
		Path        string   `arg:"" optional:"" name:"path" predictor:"file" help:"Path of the new stack."`
		ID          string   `help:"Set the ID of the stack, defaults to an UUIDv4 string."`
		Name        string   `help:"Set the name of the stack, defaults to the basename of <path>"`
		Description string   `help:"Set the description of the stack, defaults to <name>"`
		Import      []string `help:"Add 'import' block to the configuration of the new stack."`
		After       []string `help:"Add 'after' attribute to the configuration of the new stack."`
		Before      []string `help:"Add 'before' attribute to the configuration of the new stack."`
		Wants       []string `help:"Add 'wants' attribute to the configuration of the new stack."`
		WantedBy    []string `help:"Add 'wanted_by' attribute to the configuration of the new stack."`

		Watch          []string `help:"Add 'watch' attribute to the configuration of the new stack."`
		IgnoreExisting bool     `help:"Skip creation without error when the stack already exist."`
		AllTerraform   bool     `help:"Import existing Terraform Root Modules as stacks."`
		AllTerragrunt  bool     `help:"Import existing Terragrunt Modules as stacks."`
		EnsureStackIDs bool     `name:"ensure-stack-ids" help:"Set the ID of existing stacks that do not set an ID to a new UUIDv4."`
		NoGenerate     bool     `help:"Do not run code generation after creating the new stack."`
	} `cmd:"" help:"Create or import stacks."`

	Fmt struct {
		Files            []string `arg:"" optional:"true" predictor:"file" help:"List of files to be formatted."`
		Check            bool     `hidden:"" help:"Lists unformatted files but do not change them. (Exits with 0 if all is formatted, 1 otherwise)"`
		DetailedExitCode bool     `help:"Return a detailed exit code: 0 nothing changed, 1 an error happened, 2 changes were made."`
	} `cmd:"" help:"Format configuration files."`

	List struct {
		Why    bool   `help:"Shows the reason why the stack has changed."`
		Format string `default:"text" enum:"text,json,dot" help:"Output format (text, json, or dot)"`

		cloudFilterFlags
		Target   string `help:"Select the deployment target of the filtered stacks."`
		RunOrder bool   `default:"false" help:"Sort listed stacks by order of execution"`

		changeDetectionFlags
	} `cmd:"" help:"List stacks."`

	Run struct {
		runCommandFlags `envprefix:"TM_ARG_RUN_"`
		runSafeguardsCliSpec
		outputsSharingFlags
	} `cmd:"" help:"Run command in the stacks"`

	Generate struct {
		Parallel         int  `env:"TM_ARG_GENERATE_PARALLEL" short:"j" optional:"true" help:"Set the parallelism of code generation"`
		DetailedExitCode bool `default:"false" help:"Return a detailed exit code: 0 nothing changed, 1 an error happened, 2 changes were made."`
	} `cmd:"" help:"Run Code Generation in stacks."`

	Script struct {
		List struct{} `cmd:"" help:"List scripts."`
		Tree struct{} `cmd:"" help:"Dump a tree of scripts."`
		Info struct {
			Cmds []string `arg:"" optional:"true" passthrough:"" help:"Script to show info for."`
		} `cmd:"" help:"Show detailed information about a script"`
		Run struct {
			runScriptFlags `envprefix:"TM_ARG_RUN_"`
			runSafeguardsCliSpec
			outputsSharingFlags
		} `cmd:"" help:"Run a Terramate Script in stacks."`
	} `cmd:"" help:"Use Terramate Scripts"`

	Debug struct {
		Show struct {
			Metadata struct {
				cloudFilterFlags
			} `cmd:"" help:"Show metadata available in stacks."`
			Globals struct {
				cloudFilterFlags
			} `cmd:"" help:"Show globals available in stacks."`
			GenerateOrigins struct {
				cloudFilterFlags
			} `cmd:"" help:"Show details about generated code in stacks."`
			RuntimeEnv struct {
				cloudFilterFlags
			} `cmd:"" help:"Show available run-time environment variables (ENV) in stacks."`
		} `cmd:"" help:"Show configuration details of stacks."`
	} `cmd:"" help:"Debug Terramate configuration."`

	Cloud struct {
		Login struct {
			Google bool `optional:"true" help:"authenticate with google credentials"`
			Github bool `optional:"true" help:"authenticate with github credentials"`
			SSO    bool `optional:"true" help:"authenticate with SSO credentials"`
		} `cmd:"" help:"Sign in to Terramate Cloud."`
		Info  struct{} `cmd:"" help:"Show your current Terramate Cloud login status."`
		Drift struct {
			Show struct {
				Target string `help:"Show stacks from the given deployment target."`
			} `cmd:"" help:"Show the current drift of a stack."`
		} `cmd:"" help:"Interact with Terramate Cloud Drift Detection."`
	} `cmd:"" help:"Interact with Terramate Cloud"`

	Trigger struct {
		Stack        string `arg:"" optional:"true" name:"stack" predictor:"file" help:"The stacks path."`
		Recursive    bool   `default:"false" help:"Recursively triggers all child stacks of the given path"`
		Change       bool   `default:"false" help:"Trigger stacks as changed"`
		IgnoreChange bool   `default:"false" help:"Trigger stacks to be ignored by change detection"`
		Reason       string `default:"" name:"reason" help:"Set a reason for triggering the stack."`
		cloudFilterFlags
	} `cmd:"" hidden:""  help:"Mark a stack as changed so it will be triggered in Change Detection. (DEPRECATED)"`

	Experimental struct {
		Clone struct {
			SrcDir          string `arg:"" name:"srcdir" predictor:"file" help:"Path of the stack being cloned."`
			DestDir         string `arg:"" name:"destdir" predictor:"file" help:"Path of the new stack."`
			SkipChildStacks bool   `default:"false" help:"Do not clone nested child stacks."`
			NoGenerate      bool   `help:"Do not run code generation after cloning the stacks."`
		} `cmd:"" help:"Clone a stack."`

		Trigger struct {
			Stack        string `arg:"" optional:"true" name:"stack" predictor:"file" help:"The stacks path."`
			Recursive    bool   `default:"false" help:"Recursively triggers all child stacks of the given path"`
			Change       bool   `default:"false" help:"Trigger stacks as changed"`
			IgnoreChange bool   `default:"false" help:"Trigger stacks to be ignored by change detection"`
			Reason       string `default:"" name:"reason" help:"Set a reason for triggering the stack."`
			cloudFilterFlags
		} `cmd:"" hidden:"" help:"Mark a stack as changed so it will be triggered in Change Detection. (DEPRECATED)"`

		RunGraph struct {
			Outfile string `short:"o" predictor:"file" default:"" help:"Output .dot file"`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\""`
		} `cmd:"" help:"Generate a graph of the execution order"`

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
			Login struct{} `cmd:"" help:"login for cloud.terramate.io  (DEPRECATED)"`
			Info  struct{} `cmd:"" help:"cloud information status (DEPRECATED)"`
			Drift struct {
				Show struct {
				} `cmd:"" help:"show drifts  (DEPRECATED)"`
			} `cmd:"" help:"manage cloud drifts  (DEPRECATED)"`
		} `cmd:"" hidden:"" help:"Terramate Cloud commands (DEPRECATED)"`
	} `cmd:"" help:"Use experimental features."`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions."`

	Version struct{} `cmd:"" help:"Show Terramate version"`
}

type globalCliFlags struct {
	VersionFlag    bool     `hidden:"true" name:"version" help:"Show Terramate version."`
	Chdir          string   `env:"CHDIR" short:"C" optional:"true" predictor:"file" help:"Set working directory."`
	GitChangeBase  string   `env:"GIT_CHANGE_BASE" short:"B" optional:"true" help:"Set git base reference for computing changes."`
	Changed        bool     `env:"CHANGED" short:"c" optional:"true" help:"Filter stacks based on changes made in git."`
	Tags           []string `env:"TAGS" optional:"true" sep:"none" help:"Filter stacks by tags."`
	NoTags         []string `env:"NO_TAGS" optional:"true" sep:"," help:"Filter stacks by tags not being set."`
	LogLevel       string   `env:"LOG_LEVEL" optional:"true" default:"warn" enum:"disabled,trace,debug,info,warn,error,fatal" help:"Log level to use: 'disabled', 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'."`
	LogFmt         string   `env:"LOG_FMT" optional:"true" default:"console" enum:"console,text,json" help:"Log format to use: 'console', 'text', or 'json'."`
	LogDestination string   `env:"LOG_DESTINATION" optional:"true" default:"stderr" enum:"stderr,stdout" help:"Destination channel of log messages: 'stderr' or 'stdout'."`
	Quiet          bool     `env:"QUIET" optional:"false" help:"Disable outputs."`
	Verbose        int      `env:"VERBOSE" short:"v" optional:"true" default:"0" type:"counter" help:"Increase verboseness of output"`
}

type runSafeguardsCliSpec struct {
	// Note: The `name` and `short` are being used to define the -X flag without longer version.
	DisableSafeguardsAll            bool               `default:"false" name:"disable-safeguards=all" short:"X" help:"Disable all safeguards."`
	DisableSafeguards               safeguard.Keywords `env:"TM_DISABLE_SAFEGUARDS" enum:"git,all,none,git-untracked,git-uncommitted,outdated-code,git-out-of-sync" help:"Disable specific safeguards: 'all', 'none', 'git', 'git-untracked', 'git-uncommitted', 'git-out-of-sync', and/or 'outdated-code'."`
	DeprecatedDisableCheckGenCode   bool               `hidden:"" default:"false" name:"disable-check-gen-code" env:"TM_DISABLE_CHECK_GEN_CODE" help:"Disable outdated generated code check (DEPRECATED)."`
	DeprecatedDisableCheckGitRemote bool               `hidden:"" default:"false" name:"disable-check-git-remote" env:"TM_DISABLE_CHECK_GIT_REMOTE" help:"Disable checking if local default branch is updated with remote (DEPRECATED)."`
}

type deprecatedGlobalSafeguardsCliSpec struct {
	DeprecatedDisableCheckGitUntracked   bool `hidden:"true" optional:"true" name:"disable-check-git-untracked" default:"false" env:"TM_DISABLE_CHECK_GIT_UNTRACKED" help:"Disable git check for untracked files (DEPRECATED)."`
	DeprecatedDisableCheckGitUncommitted bool `hidden:"true" optional:"true" name:"disable-check-git-uncommitted" default:"false" env:"TM_DISABLE_CHECK_GIT_UNCOMMITTED" help:"Disable git check for uncommitted files (DEPRECATED)."`
}

type cloudFilterFlags struct {
	ExperimentalStatus string `hidden:"" help:"Filter by status (Deprecated)"`
	CloudStatus        string `hidden:""`
	Status             string `help:"Filter by Terramate Cloud status of the stack."`
	DeploymentStatus   string `help:"Filter by Terramate Cloud deployment status of the stack"`
	DriftStatus        string `help:"Filter by Terramate Cloud drift status of the stack"`
}

type changeDetectionFlags struct {
	EnableChangeDetection  []string `help:"Enable specific change detection modes" enum:"git-untracked,git-uncommitted"`
	DisableChangeDetection []string `help:"Disable specific change detection modes" enum:"git-untracked,git-uncommitted"`
}

type outputsSharingFlags struct {
	IncludeOutputDependencies bool `help:"Include stacks that are dependencies of the selected stacks. (requires outputs-sharing experiment enabled)"`
	OnlyOutputDependencies    bool `help:"Only include stacks that are dependencies of the selected stacks. (requires outputs-sharing experiment enabled)"`
}

type cloudTargetFlags struct {
	Target     string `env:"TARGET" help:"Set the deployment target for stacks synchronized to Terramate Cloud."`
	FromTarget string `env:"FROM_TARGET" help:"Migrate stacks from given deployment target."`
}

type cloudSyncFlags struct {
	CloudSyncDeployment  bool `hidden:""`
	SyncDeployment       bool `env:"SYNC_DEPLOYMENT" default:"false" help:"Synchronize the command as a new deployment to Terramate Cloud."`
	CloudSyncDriftStatus bool `hidden:""`
	SyncDriftStatus      bool `env:"SYNC_DRIFT_STATUS" default:"false" help:"Synchronize the command as a new drift run to Terramate Cloud."`
	CloudSyncPreview     bool `hidden:""`
	SyncPreview          bool `env:"SYNC_PREVIEW" default:"false" help:"Synchronize the command as a new preview to Terramate Cloud."`

	CloudSyncLayer             preview.Layer `hidden:""`
	Layer                      preview.Layer `env:"LAYER" default:"" help:"Set a customer layer for synchronizing a preview to Terramate Cloud."`
	CloudSyncTerraformPlanFile string        `hidden:""`
	PlanRenderTimeout          int           `env:"PLAN_RENDER_TIMEOUT" default:"300" help:"Timeout (in seconds) for internal commands that render changes from plan files."`
}

type commonRunFlags struct {
	NoRecursive     bool `env:"NO_RECURSIVE" default:"false" help:"Do not recurse into nested child stacks."`
	ContinueOnError bool `env:"CONTINUE_ON_ERROR" default:"false" help:"Continue executing next stacks when a command returns an error."`
	DryRun          bool `env:"DRY_RUN" default:"false" help:"Plan the execution but do not execute it."`
	Reverse         bool `env:"REVERSE" default:"false" help:"Reverse the order of execution."`

	// Note: 0 is not the real default value here, this is just a workaround.
	// Kong doesn't support having 0 as the default value in case the flag isn't set, but K in case it's set without a value.
	// The K case is handled in the custom decoder.
	Parallel int `env:"PARALLEL" short:"j" optional:"true" help:"Run independent stacks in parallel."`
}

type runCommandFlags struct {
	cloudFilterFlags
	changeDetectionFlags
	cloudTargetFlags

	EnableSharing bool `env:"ENABLE_SHARING" help:"Enable sharing of stack outputs as stack inputs."`
	MockOnFail    bool `env:"MOCK_ON_FAIL" help:"Mock the output values if command fails."`

	cloudSyncFlags

	TerraformPlanFile string `env:"TERRAFORM_PLAN_FILE" default:"" help:"Add details of the Terraform Plan file to the synchronization to Terramate Cloud."`
	TofuPlanFile      string `env:"TOFU_PLAN_FILE" default:"" help:"Add details of the OpenTofu Plan file to the synchronization to Terramate Cloud."`
	DebugPreviewURL   string `hidden:"true" default:"" help:"Create a debug preview URL to Terramate Cloud details."`

	commonRunFlags

	Eval       bool     `env:"EVAL" default:"false" help:"Evaluate command arguments as HCL strings interpolating Globals, Functions and Metadata."`
	Terragrunt bool     `env:"TERRAGRUNT" default:"false" help:"Use terragrunt when generating planfile for Terramate Cloud sync."`
	Command    []string `arg:"" name:"cmd" predictor:"file" passthrough:"" help:"Command to execute"`
}

type runScriptFlags struct {
	cloudFilterFlags
	changeDetectionFlags
	cloudTargetFlags
	commonRunFlags

	Cmds []string `arg:"" optional:"true" passthrough:"" help:"Script to execute."`
}

func migrateFlagAliases(parsedArgs *FlagSpec) {
	// list
	migrateStringFlag(&parsedArgs.List.Status, parsedArgs.List.CloudStatus)

	// run
	migrateStringFlag(&parsedArgs.Run.Status, parsedArgs.Run.CloudStatus)
	migrateBoolFlag(&parsedArgs.Run.SyncDeployment, parsedArgs.Run.CloudSyncDeployment)
	migrateBoolFlag(&parsedArgs.Run.SyncDriftStatus, parsedArgs.Run.CloudSyncDriftStatus)
	migrateBoolFlag(&parsedArgs.Run.SyncPreview, parsedArgs.Run.CloudSyncPreview)
	migrateStringFlag(&parsedArgs.Run.TerraformPlanFile, parsedArgs.Run.CloudSyncTerraformPlanFile)
	if parsedArgs.Run.CloudSyncLayer != "" && parsedArgs.Run.Layer == "" {
		parsedArgs.Run.Layer = parsedArgs.Run.CloudSyncLayer
	}

	// script run
	migrateStringFlag(&parsedArgs.Script.Run.Status, parsedArgs.Script.Run.CloudStatus)

	// experimental trigger
	migrateStringFlag(&parsedArgs.Experimental.Trigger.Status, parsedArgs.Experimental.Trigger.CloudStatus)

	// trigger
	migrateStringFlag(&parsedArgs.Trigger.Status, parsedArgs.Trigger.CloudStatus)
}

func migrateStringFlag(flag *string, alias string) {
	if alias != "" && *flag == "" {
		*flag = alias
	}
}

func migrateBoolFlag(flag *bool, alias bool) {
	if alias && !*flag {
		*flag = alias
	}
}

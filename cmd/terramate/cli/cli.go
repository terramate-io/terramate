// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	prj "github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/run"
	"github.com/mineiros-io/terramate/run/dag"

	"github.com/alecthomas/kong"
	"github.com/emicklei/dot"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/posener/complete"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/willabides/kongplete"
)

const (
	// ErrOutdatedLocalRev indicates the local revision is outdated.
	ErrOutdatedLocalRev errors.Kind = "outdated local revision"
	// ErrInit indicates a failure to initialize stacks.
	ErrInit errors.Kind = "failed to initialize all stacks"
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
)

type cliSpec struct {
	Version       struct{} `cmd:"" help:"Terramate version"`
	VersionFlag   bool     `name:"version" help:"Terramate version"`
	Chdir         string   `short:"C" optional:"true" predictor:"file" help:"Sets working directory"`
	GitChangeBase string   `short:"B" optional:"true" help:"Git base ref for computing changes"`
	Changed       bool     `short:"c" optional:"true" help:"Filter by changed infrastructure"`
	LogLevel      string   `optional:"true" default:"warn" enum:"trace,debug,info,warn,error,fatal" help:"Log level to use: 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'"`
	LogFmt        string   `optional:"true" default:"console" enum:"console,text,json" help:"Log format to use: 'console', 'text', or 'json'"`

	DisableCheckGitUntracked   bool `optional:"true" default:"false" help:"Disable git check for untracked files"`
	DisableCheckGitUncommitted bool `optional:"true" default:"false" help:"Disable git check for uncommitted files"`

	Create struct {
		Path        string   `arg:"" name:"path" predictor:"file" help:"Path of the new stack relative to the working dir"`
		ID          string   `help:"ID of the stack, defaults to UUID"`
		Name        string   `help:"Name of the stack, defaults to stack dir base name"`
		Description string   `help:"Description of the stack, defaults to the stack name"`
		Import      []string `help:"Add import block for the given path on the stack"`
	} `cmd:"" help:"Creates a stack on the project"`

	Fmt struct {
		Check bool `help:"Lists unformatted files, exit with 0 if all is formatted, 1 otherwise"`
	} `cmd:"" help:"Format all files inside dir recursively"`

	List struct {
		Why bool `help:"Shows the reason why the stack has changed"`
	} `cmd:"" help:"List stacks"`

	Run struct {
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
		Clone struct {
			SrcDir  string `arg:"" name:"srcdir" predictor:"file" help:"Path of the stack being cloned"`
			DestDir string `arg:"" name:"destdir" predictor:"file" help:"Path of the new stack"`
		} `cmd:"" help:"Clones a stack"`

		Metadata struct{} `cmd:"" help:"Shows metadata available on the project"`

		Globals struct {
		} `cmd:"" help:"List globals for all stacks"`

		RunGraph struct {
			Outfile string `short:"o" predictor:"file" default:"" help:"Output .dot file"`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\""`
		} `cmd:"" help:"Generate a graph of the execution order"`

		RunOrder struct {
			Basedir string `arg:"" optional:"true" help:"Base directory to search stacks"`
		} `cmd:"" help:"Show the topological ordering of the stacks"`

		RunEnv struct {
		} `cmd:"" help:"List run environment variables for all stacks"`
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
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) {
	configureLogging(defaultLogLevel, defaultLogFmt, stderr)
	c := newCLI(args, stdin, stdout, stderr)
	c.run()
}

type cli struct {
	ctx        *kong.Context
	parsedArgs *cliSpec
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	exit       bool
	prj        project
}

func newCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) *cli {
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
		logger.Fatal().
			Err(err).
			Msg("failed to create cli parser")
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
		logger.Debug().Msg("Get terramate version using --version.")
		fmt.Println(terramate.Version())
		return &cli{exit: true}
	}

	if err != nil {
		logger.Fatal().
			Err(err).
			Msgf("failed to parse cli args: %v", args)
	}

	configureLogging(parsedArgs.LogLevel, parsedArgs.LogFmt, stderr)
	// If we don't re-create the logger after configuring we get some
	// log entries with a mix of default fmt and selected fmt.
	logger = log.With().
		Str("action", "newCli()").
		Logger()

	switch ctx.Command() {
	case "version":
		logger.Debug().Msg("Get terramate version with version subcommand.")
		fmt.Println(terramate.Version())
		return &cli{exit: true}
	case "install-completions":
		logger.Debug().Msg("Handle `install-completions` command.")

		err := parsedArgs.InstallCompletions.Run(ctx)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("installing shell completions.")
		}
		return &cli{exit: true}
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Getwd() failed")
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
			logger.Fatal().
				Str("dir", parsedArgs.Chdir).
				Err(err).
				Msg("Changing working directory failed")
		}

		wd, err = os.Getwd()
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("Getwd() failed")
		}
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("EvalSymlinks() failed")
	}

	logger.Trace().Msg("Running in directory")

	prj, foundRoot, err := lookupProject(wd)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("failed to lookup project root")
	}

	if !foundRoot {
		logger.Fatal().
			Msg("project root not found")
	}

	logger.Trace().Msg("Set defaults from parsed command line arguments.")

	err = prj.setDefaults(&parsedArgs)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("setting configuration")
	}

	if parsedArgs.Changed && !prj.isRepo {
		logger.Fatal().
			Msg("flag --changed provided but no git repository found")
	}

	return &cli{
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		parsedArgs: &parsedArgs,
		ctx:        ctx,
		prj:        prj,
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

	logger.Debug().Msg("Handle command.")

	switch c.ctx.Command() {
	case "fmt":
		c.format()
	case "create <path>":
		c.createStack()
	case "list":
		c.printStacks()
	case "run":
		log.Fatal().Msg("no command specified")
	case "run <cmd>":
		c.runOnStacks()
	case "generate":
		c.generate(c.wd())
	case "experimental clone <srcdir> <destdir>":
		c.cloneStack()
	case "experimental globals":
		c.printStacksGlobals()
	case "experimental metadata":
		c.printMetadata()
	case "experimental run-graph":
		c.generateGraph()
	case "experimental run-order":
		c.printRunOrder()
	case "experimental run-env":
		c.printRunEnv()
	default:
		logger.Fatal().Msg("unexpected command sequence")
	}
}

func (c *cli) checkGit() {
	logger := log.With().
		Str("action", "checkGit()").
		Logger()

	if !c.prj.isRepo {
		return
	}

	logger.Trace().Msg("Check git default remote.")

	if err := c.prj.checkDefaultRemote(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Checking git default remote.")
	}

	if c.parsedArgs.GitChangeBase != "" {
		c.prj.baseRef = c.parsedArgs.GitChangeBase
	} else {
		c.prj.baseRef = c.prj.defaultBaseRef()
	}

	logger.Trace().Msg("check git default branch is updated")

	if err := c.prj.checkLocalDefaultIsUpdated(); err != nil {
		log.Fatal().
			Err(err).
			Msg("checking git default branch was updated.")
	}
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

	if err := stack.Clone(c.root(), destdir, srcdir); err != nil {
		logger.Fatal().Err(err).Msg("cloning stack")
	}

	c.log("Cloned stack %s to %s with success", srcstack, deststack)
	c.log("Generating code on the new cloned stack")

	c.generate(destdir)
}

func (c *cli) generate(workdir string) {
	report := generate.Do(c.root(), workdir)
	c.log(report.String())

	if report.HasFailures() {
		os.Exit(1)
	}
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

	cfg := c.prj.rootcfg

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

	cfg := c.prj.rootcfg

	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Git != nil {
		return cfg.Terramate.Config.Git.CheckUncommitted
	}

	return true
}

func (c *cli) gitSafeguards(checks terramate.RepoChecks, shouldAbort bool) {
	logger := log.With().
		Str("action", "gitSafeguards()").
		Logger()

	if c.parsedArgs.Run.DryRun {
		return
	}

	if c.checkGitUntracked() && len(checks.UntrackedFiles) > 0 {
		if shouldAbort {
			logger.Fatal().
				Strs("files", checks.UntrackedFiles).
				Msg("repository has untracked files")
		} else {
			logger.Warn().
				Strs("files", checks.UntrackedFiles).
				Msg("repository has untracked files")
		}

	}

	if c.checkGitUncommited() && len(checks.UncommittedFiles) > 0 {
		if shouldAbort {
			logger.Fatal().
				Strs("files", checks.UncommittedFiles).
				Msg("repository has uncommitted files")
		} else {
			logger.Warn().
				Strs("files", checks.UncommittedFiles).
				Msg("repository has uncommitted files")
		}
	}
}

func (c *cli) listStacks(mgr *terramate.Manager, isChanged bool) (*terramate.StacksReport, error) {
	if isChanged {
		log.Trace().
			Str("action", "listStacks()").
			Str("workingDir", c.wd()).
			Msg("`Changed` flag was set. List changed stacks.")
		return mgr.ListChanged()
	}
	return mgr.List()
}

func (c *cli) createStack() {
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("action", "cli.createStack()").
		Str("imports", fmt.Sprint(c.parsedArgs.Create.Import)).
		Logger()

	logger.Trace().Msg("creating stack")

	stackDir := filepath.Join(c.wd(), c.parsedArgs.Create.Path)

	stackID := c.parsedArgs.Create.ID
	if stackID == "" {

		logger.Trace().Msg("no ID provided, generating one")

		id, err := uuid.NewRandom()
		if err != nil {
			logger.Fatal().Err(err)
		}
		stackID = id.String()
	}

	stackName := c.parsedArgs.Create.Name
	if stackName == "" {
		stackName = filepath.Base(stackDir)
	}

	stackDescription := c.parsedArgs.Create.Description
	if stackDescription == "" {
		stackDescription = stackName
	}

	err := stack.Create(c.root(), stack.CreateCfg{
		Dir:         stackDir,
		ID:          stackID,
		Name:        stackName,
		Description: stackDescription,
		Imports:     c.parsedArgs.Create.Import,
	})

	if err != nil {
		logger.Fatal().Err(err).Msg("creating stack")
	}

	c.log("Created stack %s with success", c.parsedArgs.Create.Path)
	c.log("Generating code on the stack")
	c.generate(stackDir)
}

func (c *cli) format() {
	logger := log.With().
		Str("workingDir", c.wd()).
		Str("action", "format()").
		Logger()

	logger.Trace().Msg("formatting all files recursively")
	results, err := hcl.FormatTree(c.wd())
	if err != nil {
		logger.Fatal().Err(err).Msg("formatting files")
	}

	logger.Trace().Msg("listing formatted files")
	for _, res := range results {
		path := strings.TrimPrefix(res.Path(), c.wd()+string(filepath.Separator))
		c.log(path)
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
		logger.Fatal().Err(err).Msg("saving files")
	}
}

func (c *cli) printStacks() {
	logger := log.With().
		Str("action", "printStacks()").
		Logger()

	if c.parsedArgs.Changed {
		c.checkGit()
	}

	if c.parsedArgs.List.Why && !c.parsedArgs.Changed {
		logger.Fatal().
			Msg("the --why flag must be used together with --changed")
	}

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Create a new stack manager.")
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Get stack list.")
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	c.gitSafeguards(report.Checks, false)

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Print stacks.")

	for _, entry := range report.Stacks {
		stack := entry.Stack
		stackRepr, ok := c.friendlyFmtDir(stack.Path())
		if !ok {
			continue
		}

		logger.Debug().
			Stringer("stack", stack).
			Msg("Print stack.")

		if c.parsedArgs.List.Why {
			c.log("%s - %s", stackRepr, entry.Reason)
		} else {
			c.log(stackRepr)
		}
	}
}

func (c *cli) printRunEnv() {
	logger := log.With().
		Str("action", "cli.printRunEnv()").
		Str("workingDir", c.wd()).
		Logger()

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	for _, stackEntry := range c.filterStacksByWorkingDir(report.Stacks) {
		envVars, err := run.LoadEnv(c.root(), stackEntry.Stack)
		if err != nil {
			log.Fatal().Err(err).Msg("loading stack run environment")
		}

		c.log("\nstack %q:", stackEntry.Stack.Path())

		for _, envVar := range envVars {
			c.log("\t%s", envVar)
		}
	}
}

func (c *cli) generateGraph() {
	var getLabel func(s *stack.S) string

	logger := log.With().
		Str("action", "generateGraph()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().Msg("Handle graph label command line argument.")

	switch c.parsedArgs.Experimental.RunGraph.Label {
	case "stack.name":
		logger.Debug().Msg("Set label to stack name.")

		getLabel = func(s *stack.S) string { return s.Name() }
	case "stack.dir":
		logger.Debug().Msg("Set label stack directory.")

		getLabel = func(s *stack.S) string { return s.Path() }
	default:
		logger.Fatal().
			Msg("-label expects the values \"stack.name\" or \"stack.dir\"")
	}

	entries, err := terramate.ListStacks(c.root())
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("listing stacks.")
	}

	logger.Debug().Msg("Create new graph.")

	loader := stack.NewLoader(c.root())
	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New()

	visited := map[string]struct{}{}
	for _, e := range c.filterStacksByWorkingDir(entries) {
		if _, ok := visited[e.Stack.Path()]; ok {
			continue
		}

		err := run.BuildDAG(graph, c.root(), e.Stack, loader, visited)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("failed to build order tree")
		}
	}

	for _, id := range graph.IDs() {
		val, err := graph.Node(id)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("generating graph")
		}

		generateDot(dotGraph, graph, id, val.(*stack.S), getLabel)
	}

	logger.Debug().
		Msg("Set output of graph.")
	outFile := c.parsedArgs.Experimental.RunGraph.Outfile
	var out io.Writer
	if outFile == "" {
		logger.Trace().
			Msg("Set output to stdout.")
		out = c.stdout
	} else {
		logger.Trace().
			Msg("Set output to file.")
		f, err := os.Create(outFile)
		if err != nil {
			log.Fatal().
				Str("path", outFile).
				Err(err).
				Msg("opening file")
		}

		defer func() {
			if err := f.Close(); err != nil {
				log.Fatal().
					Err(err).
					Msg("closing output graph file")
			}
		}()

		out = f
	}

	logger.Debug().
		Msg("Write graph to output.")
	_, err = out.Write([]byte(dotGraph.String()))
	if err != nil {
		log.Fatal().
			Str("path", outFile).
			Err(err).
			Msg("writing output")
	}
}

func generateDot(
	dotGraph *dot.Graph,
	graph *dag.DAG,
	id dag.ID,
	stackval *stack.S,
	getLabel func(s *stack.S) string,
) {
	logger := log.With().
		Str("action", "generateDot()").
		Logger()

	parent := dotGraph.Node(getLabel(stackval))
	for _, childid := range graph.ChildrenOf(id) {
		val, err := graph.Node(childid)
		if err != nil {
			logger.Fatal().
				Err(err).
				Msg("generating dot file")
		}
		s := val.(*stack.S)
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
		logger.Fatal().
			Err(err).
			Msgf("computing selected stacks")
	}

	logger.Debug().Msg("Get run order.")
	orderedStacks, reason, err := run.Sort(c.root(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			log.Fatal().
				Err(err).
				Str("reason", reason).
				Msg("running on order")
		} else {
			log.Fatal().
				Err(err).
				Msg("failed to plan execution")
		}
	}

	for _, s := range orderedStacks {
		c.log(s.Name())
	}
}

func (c *cli) printStacksGlobals() {
	logger := log.With().
		Str("action", "printStacksGlobals()").
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	for _, stackEntry := range c.filterStacksByWorkingDir(report.Stacks) {
		meta := stack.Metadata(stackEntry.Stack)
		globals, err := stack.LoadGlobals(c.root(), meta)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("stack", meta.Path()).
				Msg("listing stacks globals: loading stack")
		}

		globalsStrRepr := globals.String()
		if globalsStrRepr == "" {
			continue
		}

		c.log("\nstack %q:", meta.Path())
		for _, line := range strings.Split(globalsStrRepr, "\n") {
			c.log("\t%s", line)
		}
	}
}

func (c *cli) printMetadata() {
	logger := log.With().
		Str("action", "printMetadata()").
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	stackEntries := c.filterStacksByWorkingDir(report.Stacks)

	if len(stackEntries) == 0 {
		return
	}

	c.log("Available metadata:")

	for _, stackEntry := range stackEntries {
		stackMeta := stack.Metadata(stackEntry.Stack)

		logger.Debug().
			Stringer("stack", stackEntry.Stack).
			Msg("Print metadata for individual stack.")

		c.log("\nstack %q:", stackMeta.Path())
		if id, ok := stackMeta.ID(); ok {
			c.log("\tterramate.stack.id=%q", id)
		}
		c.log("\tterramate.stack.name=%q", stackMeta.Name())
		c.log("\tterramate.stack.description=%q", stackMeta.Desc())
		c.log("\tterramate.stack.path.absolute=%q", stackMeta.Path())
		c.log("\tterramate.stack.path.basename=%q", stackMeta.PathBase())
		c.log("\tterramate.stack.path.relative=%q", stackMeta.RelPath())
		c.log("\tterramate.stack.path.to_root=%q", stackMeta.RelPathToRoot())
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

	cfg := c.prj.rootcfg

	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Run != nil {
		return cfg.Terramate.Config.Run.CheckGenCode
	}

	return true
}

func envVarIsSet(val string) bool {
	return val != "0" && val != "false"
}

func (c *cli) checkOutdatedGeneratedCode(stacks stack.List) {
	logger := log.With().
		Str("action", "checkOutdatedGeneratedCode()").
		Logger()

	if !c.checkGenCode() {
		logger.Trace().Msg("outdated generated code check is disabled")
		return
	}

	logger.Trace().Msg("Checking if any stack has outdated code.")

	hasOutdated := false
	for _, stack := range stacks {
		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Trace().Msg("checking stack for outdated code")

		outdated, err := generate.CheckStack(c.root(), stack)
		if err != nil {
			logger.Fatal().Err(err).Msg("checking stack for outdated code")
		}

		if len(outdated) > 0 {
			hasOutdated = true
		}

		for _, filename := range outdated {
			logger.Error().
				Str("filename", filename).
				Msg("outdated code found")
		}
	}

	if hasOutdated {
		logger.Fatal().
			Err(errors.E(ErrOutdatedGenCodeDetected)).
			Msg("please run: 'terramate generate' to update generated code")
	}
}

func (c *cli) checkGitRemote() bool {
	if c.parsedArgs.Run.DisableCheckGitRemote {
		return false
	}

	if disableCheck, ok := os.LookupEnv("TM_DISABLE_CHECK_GIT_REMOTE"); ok {
		if envVarIsSet(disableCheck) {
			return false
		}
	}

	cfg := c.prj.rootcfg

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

	if c.checkGitRemote() {
		c.checkGit()
	}

	if len(c.parsedArgs.Run.Command) == 0 {
		logger.Fatal().Msgf("run expects a cmd")
	}

	var stacks stack.List

	if c.parsedArgs.Run.NoRecursive {
		st, found, err := stack.TryLoad(c.root(), c.wd())
		if err != nil {
			logger.Fatal().
				Err(err).
				Msg("loading stack in current directory")
		}

		if !found {
			logger.Fatal().
				Msg("--no-recursive provided but no stack found in the current directory")
		}

		stacks = append(stacks, st)
	} else {
		var err error
		stacks, err = c.computeSelectedStacks(true)
		if err != nil {
			logger.Fatal().
				Err(err).
				Msg("computing selected stacks")
		}
	}

	c.checkOutdatedGeneratedCode(stacks)

	logger.Trace().Msg("Get order of stacks to run command on.")

	orderedStacks, reason, err := run.Sort(c.root(), stacks)
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			logger.Fatal().
				Str("reason", reason).
				Err(err).
				Msg("running in order")
		} else {
			log.Fatal().
				Err(err).
				Msg("failed to plan execution")
		}
	}

	if c.parsedArgs.Run.Reverse {
		logger.Trace().Msg("Reversing stacks order.")
		stack.Reverse(orderedStacks)
	}

	if c.parsedArgs.Run.DryRun {
		logger.Trace().
			Msg("Do a dry run - get order without actually running command.")
		if len(orderedStacks) > 0 {
			c.log("The stacks will be executed using order below:")

			for i, s := range orderedStacks {
				stackdir, _ := c.friendlyFmtDir(s.Path())
				c.log("\t%d. %s (%s)", i, s.Name(), stackdir)
			}
		} else {
			c.log("No stacks will be executed.")
		}

		return
	}

	logger.Info().Msg("Running on selected stacks")

	err = run.Exec(
		c.root(),
		orderedStacks,
		c.parsedArgs.Run.Command,
		c.stdin,
		c.stdout,
		c.stderr,
		c.parsedArgs.Run.ContinueOnError,
	)

	if err != nil {

		logger.Warn().Msg("one or more commands failed")

		var errs *errors.List
		if errors.As(err, &errs) {
			for _, err := range errs.Errors() {
				logger.Warn().Err(err).Send()
			}
		} else {
			logger.Warn().Err(err).Send()
		}

		os.Exit(1)
	}
}

func (c *cli) wd() string   { return c.prj.wd }
func (c *cli) root() string { return c.prj.root }

func (c *cli) log(format string, args ...interface{}) {
	fmt.Fprintln(c.stdout, fmt.Sprintf(format, args...))
}

func (c *cli) friendlyFmtDir(dir string) (string, bool) {
	return prj.FriendlyFmtDir(c.root(), c.wd(), dir)
}

func (c *cli) computeSelectedStacks(ensureCleanRepo bool) (stack.List, error) {
	logger := log.With().
		Str("action", "computeSelectedStacks()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().Msg("Create new terramate manager.")

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().Msg("Get list of stacks.")

	report, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return nil, err
	}

	c.gitSafeguards(report.Checks, ensureCleanRepo)

	logger.Trace().Msg("Filter stacks by working directory.")

	entries := c.filterStacksByWorkingDir(report.Stacks)
	stacks := make(stack.List, len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack
	}

	stacks, err = mgr.AddWantedOf(stacks)
	if err != nil {
		return nil, fmt.Errorf("adding wanted stacks: %w", err)
	}

	return stacks, nil
}

func (c *cli) filterStacksByWorkingDir(stacks []terramate.Entry) []terramate.Entry {
	logger := log.With().
		Str("action", "filterStacksByWorkingDir()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().
		Msg("Get relative working directory.")
	relwd := prj.PrjAbsPath(c.root(), c.wd())

	logger.Trace().
		Msg("Get filtered stacks.")
	filtered := []terramate.Entry{}
	for _, e := range stacks {
		if strings.HasPrefix(e.Stack.Path(), relwd) {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

func (c cli) checkVersion() {
	logger := log.With().
		Str("action", "cli.checkVersion()").
		Str("root", c.root()).
		Logger()

	logger.Trace().Msg("checking if terramate version satisfies project constraint")

	rootcfg := c.prj.rootcfg

	if rootcfg.Terramate == nil {
		logger.Debug().Msg("project root has no config, skipping version check")
		return
	}

	if rootcfg.Terramate.RequiredVersion == "" {
		logger.Debug().Msg("project root config has no required_version, skipping version check")
		return
	}

	if err := terramate.CheckVersion(rootcfg.Terramate.RequiredVersion); err != nil {
		logger.Fatal().Err(err).Send()
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
		return nil, fmt.Errorf("dir %q is not a git repository", basedir)
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

	rootcfg, rootCfgPath, rootfound, err := config.TryLoadRootConfig(wd)
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

			if err != nil {
				return project{}, false, fmt.Errorf("getting absolute path of %q: %w", gitdir, err)
			}

			logger.Trace().Msg("Evaluate symbolic links.")

			gitabs, err = filepath.EvalSymlinks(gitabs)
			if err != nil {
				return project{}, false, fmt.Errorf("failed evaluating symlinks of %q: %w",
					gitabs, err)
			}

			rootdir := filepath.Dir(gitabs)

			if rootfound && strings.HasPrefix(rootCfgPath, rootdir) && rootCfgPath != rootdir {
				logger.Warn().
					Str("rootConfig", rootCfgPath).
					Str("projectRoot", rootdir).
					Err(errors.E(ErrRootCfgInvalidDir)).
					Msg("the config will be ignored")
			}

			logger.Trace().Msg("Load root config.")

			cfg, err := hcl.ParseDir(rootdir, rootdir)
			if err != nil {
				return project{}, false, err
			}

			prj.isRepo = true
			prj.rootcfg = cfg
			prj.root = rootdir
			prj.git.wrapper = gw

			return prj, true, nil
		}
	}

	if !rootfound {
		return project{}, false, nil
	}

	prj.root = rootCfgPath
	prj.rootcfg = rootcfg
	return prj, true, nil

}

func configureLogging(logLevel string, logFmt string, output io.Writer) {
	zloglevel, err := zerolog.ParseLevel(logLevel)

	if err != nil {
		zloglevel = zerolog.FatalLevel
	}

	zerolog.SetGlobalLevel(zloglevel)

	if logFmt == "json" {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(output)
	} else if logFmt == "text" { // no color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339})
	} else { // default: console mode using color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: false, TimeFormat: time.RFC3339})
	}
}

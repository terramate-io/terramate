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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mineiros-io/terramate/generate"
	prj "github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/run"
	"github.com/mineiros-io/terramate/run/dag"

	"github.com/alecthomas/kong"
	"github.com/emicklei/dot"
	"github.com/madlambda/spells/errutil"
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
	ErrOutdatedLocalRev        errutil.Error = "outdated local revision"
	ErrNoDefaultRemoteConfig   errutil.Error = "repository must have a configured origin/main"
	ErrInit                    errutil.Error = "failed to initialize all stacks"
	ErrOutdatedGenCodeDetected errutil.Error = "outdated generated code detected"
)

const (
	defaultRemote        = "origin"
	defaultBranch        = "main"
	defaultBaseRef       = defaultRemote + "/" + defaultBranch
	defaultBranchBaseRef = "HEAD^"
)

type cliSpec struct {
	Version       struct{} `cmd:"" help:"Terramate version."`
	Chdir         string   `short:"C" optional:"true" help:"sets working directory."`
	GitChangeBase string   `short:"B" optional:"true" help:"git base ref for computing changes."`
	Changed       bool     `short:"c" optional:"true" help:"filter by changed infrastructure"`
	LogLevel      string   `optional:"true" default:"info" enum:"trace,debug,info,warn,error,fatal" help:"Log level to use: 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'"`
	LogFmt        string   `optional:"true" default:"console" enum:"console,text,json" help:"Log format to use: 'console', 'text', or 'json'."`

	Run struct {
		Quiet   bool     `short:"q" help:"Don't print any information other than the command output."`
		DryRun  bool     `default:"false" help:"plan the execution but do not execute it"`
		Command []string `arg:"" name:"cmd" passthrough:"" help:"command to execute."`
	} `cmd:"" help:"Run command in the stacks."`

	Plan struct {
		Graph struct {
			Outfile string `short:"o" default:"" help:"output .dot file."`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\"."`
		} `cmd:"" help:"generate a graph of the execution order."`

		RunOrder struct {
			Basedir string `arg:"" optional:"true" help:"base directory to search stacks."`
		} `cmd:"" help:"show the topological ordering of the stacks"`
	} `cmd:"" help:"plan execution."`

	Stacks struct {
		Init struct {
			StackDirs []string `arg:"" name:"paths" optional:"true" help:"the stack directory (current directory if not set)."`
			Force     bool     `help:"force initialization."`
		} `cmd:"" help:"Initialize a stack."`

		List struct {
			Why bool `help:"Shows the reason why the stack has changed."`
		} `cmd:"" help:"List stacks."`

		Globals struct {
		} `cmd:"" help:"list globals for all stacks."`
	} `cmd:"" help:"stack related commands."`

	Generate struct{} `cmd:"" help:"Generate terraform code for stacks."`
	Metadata struct{} `cmd:"" help:"shows metadata available on the project"`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
}

// Exec will execute terramate with the provided flags defined on args.
// Only flags should be on the args slice.

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
	c := newCLI(args, stdin, stdout, stderr)
	c.run()
}

type project struct {
	root    string
	wd      string
	isRepo  bool
	rootcfg hcl.Config
	baseRef string
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

func newCLI(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) *cli {
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
		kongplete.WithPredictor("cli", complete.PredictAnything),
	)

	ctx, err := parser.Parse(args)

	if kongExit && kongExitStatus == 0 {
		return &cli{exit: true}
	}

	if err != nil {
		logger.Fatal().
			Err(err).
			Msgf("failed to parse cli args: %v", args)
	}

	configureLogging(parsedArgs.LogLevel, parsedArgs.LogFmt, stderr)

	if ctx.Command() == "version" {
		logger.Debug().
			Msg("Get terramate version.")
		fmt.Println(terramate.Version())
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

	logger.Trace().
		Msg("Running in directory")

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

	logger.Trace().
		Msg("Set defaults from parsed command line arguments.")
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
		Str("workingDir", c.wd()).
		Logger()

	if c.parsedArgs.Changed {
		logger.Trace().
			Msg("`Changed` flag was set.")

		logger.Trace().
			Msg("Create new git wrapper.")
		git, err := newGit(c.root(), true)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("creating git wrapper.")
		}

		logger.Trace().
			Msg("Check git default remote.")
		if err := c.checkDefaultRemote(git); err != nil {
			log.Fatal().
				Err(err).
				Msg("Checking git default remote.")
		}

		logger.Trace().
			Msg("Check git default branch was updated.")
		if err := c.checkLocalDefaultIsUpdated(git); err != nil {
			log.Fatal().
				Err(err).
				Msg("checking git default branch was updated.")
		}
	}

	logger.Debug().
		Msg("Handle input command.")
	switch c.ctx.Command() {
	case "plan graph":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Handle `plan graph`.")
		c.generateGraph()
	case "plan run-order":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Print run-order.")
		c.printRunOrder()
	case "stacks init":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Handle stacks init command.")
		c.initStack([]string{c.wd()})
	case "stacks list":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Print list of stacks.")
		c.printStacks()
	case "stacks init <paths>":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Handle stacks init <paths> command.")
		c.initStack(c.parsedArgs.Stacks.Init.StackDirs)
	case "stacks globals":
		log.Trace().
			Str("actionContext", "cli()").
			Msg("Handle stacks global command.")
		c.printStacksGlobals()
	case "run":
		logger.Debug().
			Msg("Handle `run` command.")
		if len(c.parsedArgs.Run.Command) == 0 {
			log.Fatal().
				Msg("no command specified")
		}
		fallthrough
	case "run <cmd>":
		logger.Debug().
			Msg("Handle `run <cmd>` command.")
		c.runOnStacks()
	case "generate":
		logger.Debug().
			Msg("Handle `generate` command.")
		err := generate.Do(c.root(), c.wd())
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("generating code.")
		}
	case "metadata":
		logger.Debug().
			Msg("Handle `metadata` command.")
		c.printMetadata()
	case "install-completions":
		logger.Debug().
			Msg("Handle `install-completions` command.")
		err := c.parsedArgs.InstallCompletions.Run(c.ctx)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("installing shell completions.")
		}
	default:
		log.Fatal().
			Msgf("unexpected command sequence: %s", c.ctx.Command())
	}
}

func (c *cli) initStack(dirs []string) {
	var errmsgs []string

	logger := log.With().
		Str("action", "initStack()").
		Logger()

	logger.Debug().
		Msg("Init stacks.")
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			log.Trace().
				Str("stack", fmt.Sprintf("%s%s", c.wd(), strings.Trim(d, "."))).
				Msg("Make file path absolute.")
			d = filepath.Join(c.wd(), d)
		}

		log.Debug().
			Str("stack", fmt.Sprintf("%s%s", c.wd(), strings.Trim(d, "."))).
			Msg("Init stack.")

		err := terramate.Init(c.root(), d, c.parsedArgs.Stacks.Init.Force)
		if err != nil {
			c.logerr("warn: failed to initialize stack: %v", err)
			errmsgs = append(errmsgs, err.Error())
		}
	}

	if len(errmsgs) > 0 {
		log.Fatal().
			Err(ErrInit).
			Send()
	}
}

func (c *cli) listStacks(mgr *terramate.Manager, isChanged bool) ([]terramate.Entry, error) {
	if isChanged {
		log.Trace().
			Str("action", "listStacks()").
			Str("workingDir", c.wd()).
			Msg("`Changed` flag was set. List changed stacks.")
		return mgr.ListChanged()
	}
	return mgr.List()
}

func (c *cli) printStacks() {
	logger := log.With().
		Str("action", "printStacks()").
		Logger()

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Create a new stack manager.")
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Get stack list.")
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil && !errors.Is(err, terramate.ErrDirtyRepo) {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	logger.Trace().
		Str("workingDir", c.wd()).
		Msg("Print stacks.")
	for _, entry := range entries {
		stack := entry.Stack
		stackRepr, ok := c.friendlyFmtDir(stack.PrjAbsPath())
		if !ok {
			continue
		}

		logger.Debug().
			Stringer("stack", stack).
			Msg("Print stack.")

		if c.parsedArgs.Stacks.List.Why {
			c.log("%s - %s", stackRepr, entry.Reason)
		} else {
			c.log(stackRepr)
		}
	}
}

func (c *cli) generateGraph() {
	var getLabel func(s stack.S) string

	logger := log.With().
		Str("action", "generateGraph()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().
		Msg("Handle graph label command line argument.")
	switch c.parsedArgs.Plan.Graph.Label {
	case "stack.name":
		logger.Debug().
			Msg("Set label to stack name.")
		getLabel = func(s stack.S) string { return s.Name() }
	case "stack.dir":
		logger.Debug().
			Msg("Set label stack directory.")
		getLabel = func(s stack.S) string { return s.PrjAbsPath() }
	default:
		logger.Fatal().
			Msg("-label expects the values \"stack.name\" or \"stack.dir\"")
	}
	entries, err := terramate.ListStacks(c.root())
	if err != nil && !errors.Is(err, terramate.ErrDirtyRepo) {
		logger.Fatal().
			Err(err).
			Msg("listing stacks.")
	}

	logger.Debug().
		Msg("Create new graph.")
	loader := stack.NewLoader(c.root())
	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New()

	visited := map[string]struct{}{}
	for _, e := range c.filterStacksByWorkingDir(entries) {
		if _, ok := visited[e.Stack.PrjAbsPath()]; ok {
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

		generateDot(dotGraph, graph, id, val.(stack.S), getLabel)
	}

	logger.Debug().
		Msg("Set output of graph.")
	outFile := c.parsedArgs.Plan.Graph.Outfile
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

		defer f.Close()

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
	stackval stack.S,
	getLabel func(s stack.S) string,
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
		s := val.(stack.S)
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
	order, reason, err := run.Sort(c.root(), stacks, c.parsedArgs.Changed)
	if err != nil {
		if errors.Is(err, dag.ErrCycleDetected) {
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

	for _, s := range order {
		c.log("%s", s)
	}
}

func (c *cli) printStacksGlobals() {
	logger := log.With().
		Str("action", "printStacksGlobals()").
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	stackEntries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil && !errors.Is(err, terramate.ErrDirtyRepo) {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	for _, stackEntry := range c.filterStacksByWorkingDir(stackEntries) {
		stack := stackEntry.Stack
		stackMeta := stack.Meta()
		globals, err := terramate.LoadStackGlobals(c.root(), stackMeta)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("stack", stackMeta.Path).
				Msg("listing stacks globals: loading stack")
		}

		globalsStrRepr := globals.String()
		if globalsStrRepr == "" {
			continue
		}

		c.log("\nstack %q:", stackMeta.Path)
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
	stackEntries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil && !errors.Is(err, terramate.ErrDirtyRepo) {
		logger.Fatal().
			Err(err).
			Msg("listing stacks")
	}

	stackEntries = c.filterStacksByWorkingDir(stackEntries)

	if len(stackEntries) == 0 {
		return
	}

	c.log("Available metadata:")

	for _, stackEntry := range stackEntries {
		stack := stackEntry.Stack
		stackMeta := stack.Meta()

		logger.Debug().
			Stringer("stack", stack).
			Msg("Print metadata for individual stack.")

		c.log("\nstack %q:", stack.PrjAbsPath())
		c.log("\tterramate.name=%q", stackMeta.Name)
		c.log("\tterramate.path=%q", stackMeta.Path)
		c.log("\tterramate.description=%q", stackMeta.Description)
	}
}

func (c *cli) runOnStacks() {
	logger := log.With().
		Str("action", "runOnStacks()").
		Str("workingDir", c.wd()).
		Logger()

	stacks, err := c.computeSelectedStacks(true)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msgf("computing selected stacks")
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
			Err(ErrOutdatedGenCodeDetected).
			Msg("please run: 'terramate generate' to update generated code")
	}

	logger.Trace().Msg("Get order of stacks to run command on.")

	order, reason, err := run.Sort(c.root(), stacks, c.parsedArgs.Changed)
	if err != nil {
		if errors.Is(err, dag.ErrCycleDetected) {
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

	if c.parsedArgs.Run.DryRun {
		logger.Trace().
			Msg("Do a dry run - get order without actually running command.")
		if len(order) > 0 {
			c.log("The stacks will be executed using order below:")

			for i, s := range order {
				stackdir, _ := c.friendlyFmtDir(s.PrjAbsPath())
				c.log("\t%d. %s (%s)", i, s.Name(), stackdir)
			}
		} else {
			c.log("No stacks will be executed.")
		}

		return
	}

	logger.Info().
		Bool("changed", c.parsedArgs.Changed).
		Msg("Running command in stacks reachable from working directory")

	logger.Trace().Msg("Get command to run.")
	cmd := run.Cmd{
		Path:    c.parsedArgs.Run.Command[0],
		Args:    c.parsedArgs.Run.Command[1:],
		Stdin:   c.stdin,
		Stdout:  c.stdout,
		Stderr:  c.stderr,
		Environ: os.Environ(),
	}

	logger.Debug().Msg("Run command.")
	err = cmd.Run(c.root(), order)
	if err != nil {
		c.logerr("warn: failed to execute command: %v", err)
	}
}

func (c *cli) wd() string   { return c.prj.wd }
func (c *cli) root() string { return c.prj.root }

func (c *cli) log(format string, args ...interface{}) {
	fmt.Fprintln(c.stdout, fmt.Sprintf(format, args...))
}

func (c *cli) logerr(format string, args ...interface{}) {
	fmt.Fprintln(c.stderr, fmt.Sprintf(format, args...))
}

func (c *cli) checkDefaultRemote(g *git.Git) error {
	logger := log.With().
		Str("action", "checkDefaultRemote()").
		Str("stack", c.wd()).
		Logger()

	logger.Trace().
		Msg("Get list of configured git remotes.")
	remotes, err := g.Remotes()
	if err != nil {
		return fmt.Errorf("checking if remote %q exists: %v", defaultRemote, err)
	}

	var defRemote *git.Remote

	logger.Trace().
		Msg("Find default git remote.")
	for _, remote := range remotes {
		if remote.Name == defaultRemote {
			defRemote = &remote
			break
		}
	}

	if defRemote == nil {
		return fmt.Errorf(
			"%w:no default remote %q",
			ErrNoDefaultRemoteConfig,
			defaultRemote,
		)
	}

	logger.Trace().
		Msg("Find default git branch.")
	for _, branch := range defRemote.Branches {
		if branch == defaultBranch {
			return nil
		}
	}

	return fmt.Errorf(
		"%w:%q has no default branch %q,branches:%v",
		ErrNoDefaultRemoteConfig,
		defaultRemote,
		defaultBranch,
		defRemote.Branches,
	)
}

func (c *cli) checkLocalDefaultIsUpdated(g *git.Git) error {
	logger := log.With().
		Str("action", "checkLocalDefaultIsUpdated()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().
		Msg("Temporarily disabled.")

	//logger.Trace().
	//Msg("Get current git branch.")
	//branch, err := g.CurrentBranch()
	//if err != nil {
	//return fmt.Errorf("checking local branch is updated: %v", err)
	//}

	//if branch != defaultBranch {
	//return nil
	//}

	//c.logerr("current branch %q is the default branch, checking if it is updated.", branch)
	//c.logerr("retrieving info from remote branch: %s/%s ...", defaultRemote, defaultBranch)

	//logger.Trace().
	//Msg("Fetch remote reference.")
	//remoteRef, err := g.FetchRemoteRev(defaultRemote, defaultBranch)
	//if err != nil {
	//return fmt.Errorf("checking local branch %q is update: %v", branch, err)
	//}
	//c.logerr("retrieved info from remote branch: %s/%s.", defaultRemote, defaultBranch)

	//logger.Trace().
	//Msg("Get local commit ID.")
	//localCommitID, err := g.RevParse(branch)
	//if err != nil {
	//return fmt.Errorf("checking local branch %q is update: %v", branch, err)
	//}

	//localRef := git.Ref{CommitID: localCommitID}

	//if localRef.CommitID != remoteRef.CommitID {
	//return fmt.Errorf(
	//"%w: remote %s/%s=%q != local %s=%q",
	//ErrOutdatedLocalRev,
	//defaultRemote,
	//defaultBranch,
	//remoteRef.ShortCommitID(),
	//branch,
	//localRef.ShortCommitID(),
	//)

	//}

	return nil
}

func (c *cli) friendlyFmtDir(dir string) (string, bool) {
	return prj.FriendlyFmtDir(c.root(), c.wd(), dir)
}

func (c *cli) computeSelectedStacks(ensureCleanRepo bool) ([]stack.S, error) {
	logger := log.With().
		Str("action", "computeSelectedStacks()").
		Str("workingDir", c.wd()).
		Logger()

	logger.Trace().Msg("Create new terramate manager.")

	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().Msg("Get list of stacks.")

	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		if ensureCleanRepo {
			return nil, err
		}
		if !errors.Is(err, terramate.ErrDirtyRepo) {
			return nil, err
		}
	}

	logger.Trace().Msg("Filter stacks by working directory.")

	entries = c.filterStacksByWorkingDir(entries)

	stacks := make([]stack.S, len(entries))
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
		if strings.HasPrefix(e.Stack.PrjAbsPath(), relwd) {
			filtered = append(filtered, e)
		}
	}

	return filtered
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

	logger.Trace().
		Msg("Create new git wrapper.")
	gw, err := newGit(wd, false)
	if err == nil {
		logger.Trace().
			Msg("Get root of git repo.")
		gitdir, err := gw.Root()
		if err == nil {
			logger.Trace().
				Msg("Get absolute path of git directory.")
			gitabs, err := filepath.Abs(gitdir)
			if err != nil {
				return project{}, false, fmt.Errorf("getting absolute path of %q: %w", gitdir, err)
			}

			logger.Trace().
				Msg("Evaluate symbolic links.")
			gitabs, err = filepath.EvalSymlinks(gitabs)
			if err != nil {
				return project{}, false, fmt.Errorf("failed evaluating symlinks of %q: %w",
					gitabs, err)
			}

			root := filepath.Dir(gitabs)

			logger.Trace().
				Msg("Load root config.")
			cfg, _, err := config.TryLoadRootConfig(root)
			if err != nil {
				return project{}, false, err
			}

			prj.isRepo = true
			prj.rootcfg = cfg
			prj.root = root

			return prj, true, nil
		}
	}

	dir := wd

	for {
		logger.Trace().
			Msg("Load root config.")
		cfg, ok, err := config.TryLoadRootConfig(dir)
		if err != nil {
			return project{}, false, err
		}

		if ok {
			prj.root = dir
			prj.rootcfg = cfg

			return prj, true, nil
		}

		if dir == "/" {
			break
		}

		dir = filepath.Dir(dir)
	}

	return project{}, false, nil
}

func (p *project) setDefaults(parsedArgs *cliSpec) error {
	logger := log.With().
		Str("action", "setDefaults()").
		Str("workingDir", p.wd).
		Logger()

	if p.rootcfg.Terramate == nil {
		// if config has no terramate block we create one with default
		// configurations.
		logger.Trace().
			Str("configFile", p.root+"/terramate.tm.hcl").
			Msg("Create terramate block.")
		p.rootcfg.Terramate = &hcl.Terramate{}
	}

	logger.Debug().
		Str("configFile", p.root+"/terramate.tm.hcl").
		Msg("Set defaults.")
	cfg := &p.rootcfg
	if cfg.Terramate.RootConfig == nil {
		p.rootcfg.Terramate.RootConfig = &hcl.RootConfig{}
	}
	gitOpt := &cfg.Terramate.RootConfig.Git

	if gitOpt.BaseRef == "" {
		gitOpt.BaseRef = defaultBaseRef
	}

	if gitOpt.DefaultBranchBaseRef == "" {
		gitOpt.DefaultBranchBaseRef = defaultBranchBaseRef
	}

	if gitOpt.DefaultBranch == "" {
		gitOpt.DefaultBranch = defaultBranch
	}

	if gitOpt.DefaultRemote == "" {
		gitOpt.DefaultRemote = defaultRemote
	}

	baseRef := parsedArgs.GitChangeBase
	if baseRef == "" {
		baseRef = gitOpt.BaseRef
		if p.isRepo {
			logger.Trace().
				Str("configFile", p.root+"/terramate.tm.hcl").
				Msg("Create new git wrapper.")
			gw, err := newGit(p.wd, false)
			if err != nil {
				return err
			}

			logger.Trace().
				Str("configFile", p.root+"/terramate.tm.hcl").
				Msg("Get current branch.")
			branch, err := gw.CurrentBranch()
			if err != nil {
				// ON CI envs we don't have a clean way to get the branch name
				// git symbolic-ref will just fail. but we don't want a hard
				// fail on this case. We need to re-assess this, if we really
				// need the branch name, is there some other way to solve this ? etc.
				//
				// More info on git branch names on GHA:
				//
				// - https://github.com/github/feedback/discussions/5251
				// - https://stackoverflow.com/questions/58033366/how-to-get-the-current-branch-within-github-actions
				logger.Debug().
					Str("details", err.Error()).
					Msg("getting git branch name")
			} else {
				if branch == gitOpt.DefaultBranch {
					baseRef = gitOpt.DefaultBranchBaseRef
				}
			}
		}
	}

	p.baseRef = baseRef

	return nil
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

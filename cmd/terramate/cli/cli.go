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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mineiros-io/terramate/dag"
	"github.com/mineiros-io/terramate/generate"
	prj "github.com/mineiros-io/terramate/project"

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
	ErrOutdatedLocalRev      errutil.Error = "outdated local revision"
	ErrNoDefaultRemoteConfig errutil.Error = "repository must have a configured origin/main"
	ErrInit                  errutil.Error = "failed to initialize all stacks"
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
	inheritEnv bool
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	exit       bool
	prj        project
}

// New creates a new terramate command-line interface.
func New(
	args []string,
	inheritEnv bool,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) (*cli, error) {
	if len(args) == 0 {
		// WHY: avoid default kong error, print help
		args = []string{"--help"}
	}

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
		return nil, fmt.Errorf("failed to create cli parser: %v", err)
	}

	kongplete.Complete(parser,
		kongplete.WithPredictor("cli", complete.PredictAnything),
	)

	ctx, err := parser.Parse(args)

	if kongExit && kongExitStatus == 0 {
		return &cli{exit: true}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse cli args %v: %v", args, err)
	}

	logLevel := parsedArgs.LogLevel
	logFmt := parsedArgs.LogFmt

	configureLogging(logLevel, logFmt, stderr)

	log.Trace().
		Str("action", "newCli()").
		Msg("Get working directory.")
	wd := parsedArgs.Chdir
	if wd == "" {
		wd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	logger := log.With().
		Str("action", "newCli()").
		Str("stack", wd).
		Logger()

	logger.Trace().
		Msgf("Evaluate symbolic links for %q.", wd)
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return nil, fmt.Errorf("failed evaluating symlinks for %q: %w", wd, err)
	}

	logger.Trace().
		Msgf("Get absolute file path of %q.", wd)
	wd, err = filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of %q: %w", wd, err)
	}

	logger.Trace().
		Msgf("Change working directory to %q.", wd)
	err = os.Chdir(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to change working directory to %q: %w", wd, err)
	}

	logger.Trace().
		Msgf("Look up project in %q.", wd)
	prj, foundRoot, err := lookupProject(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup project root from %q: %w", wd, err)
	}

	if !foundRoot {
		return nil, fmt.Errorf("project root not found")
	}

	logger.Trace().
		Msg("Set defaults from parsed command line arguments.")
	err = prj.setDefaults(&parsedArgs)
	if err != nil {
		return nil, fmt.Errorf("setting configuration: %w", err)
	}

	if parsedArgs.Changed && !prj.isRepo {
		return nil, fmt.Errorf("flag --changed provided but no git repository found")
	}

	return &cli{
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		inheritEnv: inheritEnv,
		parsedArgs: &parsedArgs,
		ctx:        ctx,
		prj:        prj,
	}, nil
}

// Run will run terramate with the provided flags defined on args from the
// directory wd.
// Only flags should be on the args slice.

// Results will be written on stdout, according to the
// command flags. Any partial/non-critical errors will be
// written on stderr.
//
// Sometimes sub commands may be executed, the provided stdin
// will be passed to then as the sub process stdin.
//
// Each run call is completely isolated from each other (no shared state)
// as far as the parameters are not shared between the run calls.
//
// If a critical error is found an non-nil error is returned.
func (c *cli) Run() error {
	if c.exit {
		// WHY: parser called exit but with no error (like help)
		return nil
	}

	logger := log.With().
		Str("action", "run()").
		Str("stack", c.wd()).
		Logger()

	if c.parsedArgs.Changed {
		logger.Trace().
			Msg("`Changed` flag was set.")

		logger.Trace().
			Msg("Create new git wrapper.")
		git, err := newGit(c.root(), c.inheritEnv, true)
		if err != nil {
			return err
		}

		logger.Trace().
			Msg("Check git default remote.")
		if err := c.checkDefaultRemote(git); err != nil {
			return err
		}

		logger.Trace().
			Msg("Check git default branch was updated.")
		if err := c.checkLocalDefaultIsUpdated(git); err != nil {
			return err
		}
	}

	logger.Debug().
		Msg("Handle input command.")
	switch c.ctx.Command() {
	case "version":
		logger.Debug().
			Msg("Get terramate version.")
		c.log(terramate.Version())
	case "plan graph":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Handle `plan graph`.")
		return c.generateGraph()
	case "plan run-order":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Print run-order.")
		return c.printRunOrder()
	case "stacks init":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Handle stacks init command.")
		return c.initStack([]string{c.wd()})
	case "stacks list":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Print list of stacks.")
		return c.printStacks()
	case "stacks init <paths>":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Handle stacks init <paths> command.")
		return c.initStack(c.parsedArgs.Stacks.Init.StackDirs)
	case "stacks globals":
		log.Trace().
			Str("actionContext", "cli()").
			Str("stack", c.wd()).
			Msg("Handle stacks global command.")
		return c.printStacksGlobals()
	case "run":
		logger.Debug().
			Msg("Handle `run` command.")
		if len(c.parsedArgs.Run.Command) == 0 {
			return errors.New("no command specified")
		}
		fallthrough
	case "run <cmd>":
		logger.Debug().
			Msg("Handle `run <cmd>` command.")
		return c.runOnStacks()
	case "generate":
		logger.Debug().
			Msg("Handle `generate` command.")
		return generate.Do(c.root())
	case "metadata":
		logger.Debug().
			Msg("Handle `metadata` command.")
		return c.printMetadata()
	case "install-completions":
		logger.Debug().
			Msg("Handle `install-completions` command.")
		return c.parsedArgs.InstallCompletions.Run(c.ctx)
	default:
		return fmt.Errorf("unexpected command sequence: %s", c.ctx.Command())
	}

	return nil
}

func (c *cli) initStack(dirs []string) error {
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
		return ErrInit
	}

	return nil
}

func (c *cli) listStacks(mgr *terramate.Manager, isChanged bool) ([]terramate.Entry, error) {
	if isChanged {
		log.Trace().
			Str("action", "listStacks()").
			Str("stack", c.wd()).
			Msg("`Changed` flag was set. List changed stacks.")
		return mgr.ListChanged()
	}
	return mgr.List()
}

func (c *cli) printStacks() error {
	logger := log.With().
		Str("action", "printStacks()").
		Logger()

	logger.Trace().
		Str("stack", c.wd()).
		Msg("Create a new stack manager.")
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().
		Str("stack", c.wd()).
		Msg("Get stack list.")
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	logger.Trace().
		Str("stack", c.wd()).
		Msg("Print stacks.")
	for _, entry := range entries {
		stack := entry.Stack
		stackRepr, ok := c.friendlyFmtDir(stack.Dir)
		if !ok {
			continue
		}

		logger.Debug().
			Str("stack", c.wd()+stack.Dir).
			Msg("Print stack.")

		if c.parsedArgs.Stacks.List.Why {
			c.log("%s - %s", stackRepr, entry.Reason)
		} else {
			c.log(stackRepr)
		}
	}
	return nil
}

func (c *cli) generateGraph() error {
	var getLabel func(s stack.S) string

	logger := log.With().
		Str("action", "generateGraph()").
		Str("stack", c.wd()).
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
		getLabel = func(s stack.S) string { return s.Dir }
	default:
		return fmt.Errorf("-label expects the values \"stack.name\" or \"stack.dir\"")
	}
	entries, err := terramate.ListStacks(c.root())
	if err != nil {
		return err
	}

	logger.Debug().
		Msg("Create new graph.")
	loader := stack.NewLoader(c.root())
	dotGraph := dot.NewGraph(dot.Directed)
	graph := dag.New()

	visited := map[string]struct{}{}
	for _, e := range c.filterStacksByWorkingDir(entries) {
		if _, ok := visited[e.Stack.Dir]; ok {
			continue
		}

		err := terramate.BuildDAG(graph, c.root(), e.Stack, loader, visited)
		if err != nil {
			return fmt.Errorf("failed to build order tree: %w", err)
		}
	}

	for _, id := range graph.IDs() {
		val, err := graph.Node(id)
		if err != nil {
			return fmt.Errorf("generating graph: %w", err)
		}

		err = generateDot(dotGraph, graph, id, val.(stack.S), getLabel)
		if err != nil {
			return err
		}
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
			return fmt.Errorf("opening file %q: %w", outFile, err)
		}

		defer f.Close()

		out = f
	}

	logger.Debug().
		Msg("Write graph to output.")
	_, err = out.Write([]byte(dotGraph.String()))
	if err != nil {
		return fmt.Errorf("writing output to %q: %w", outFile, err)
	}

	return nil
}

func generateDot(
	dotGraph *dot.Graph,
	graph *dag.DAG,
	id dag.ID,
	stackval stack.S,
	getLabel func(s stack.S) string,
) error {
	parent := dotGraph.Node(getLabel(stackval))
	for _, childid := range graph.ChildrenOf(id) {
		val, err := graph.Node(childid)
		if err != nil {
			return fmt.Errorf("generating dot file: %w", err)
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

		err = generateDot(dotGraph, graph, childid, s, getLabel)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *cli) printRunOrder() error {
	logger := log.With().
		Str("action", "printRunOrder()").
		Str("stack", c.wd()).
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().
		Msg("Get list of stacks.")
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	logger.Trace().
		Msg("Filter stacks by working directory.")
	entries = c.filterStacksByWorkingDir(entries)

	logger.Trace().
		Msg("Create stack array.")
	stacks := make([]stack.S, len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack
	}

	logger.Debug().
		Msg("Get run order.")
	order, reason, err := terramate.RunOrder(c.root(), stacks, c.parsedArgs.Changed)
	if err != nil {
		if errors.Is(err, dag.ErrCycleDetected) {
			return fmt.Errorf("%w: reason is %s", err, reason)
		} else {
			return fmt.Errorf("failed to plan execution: %w", err)
		}
	}

	for _, s := range order {
		c.log("%s", s)
	}

	return nil
}

func (c *cli) printStacksGlobals() error {
	metadata, err := terramate.LoadMetadata(c.root())
	if err != nil {
		return fmt.Errorf("listing stacks globals: loading stacks metadata: %v", err)
	}

	for _, stackMetadata := range metadata.Stacks {
		globals, err := terramate.LoadStackGlobals(c.root(), stackMetadata)
		if err != nil {
			return fmt.Errorf(
				"listing stacks globals: loading stack %q globals: %v",
				stackMetadata.Path,
				err,
			)
		}

		globalsStrRepr := globals.String()
		if globalsStrRepr == "" {
			continue
		}

		c.log("\nstack %q:", stackMetadata.Path)
		for _, line := range strings.Split(globalsStrRepr, "\n") {
			c.log("\t%s", line)
		}
	}
	return nil
}

func (c *cli) printMetadata() error {
	logger := log.With().
		Str("action", "printMetadata()").
		Logger()

	logger.Trace().
		Str("stack", c.wd()).
		Msg("Load metadata.")
	metadata, err := terramate.LoadMetadata(c.root())
	if err != nil {
		return err
	}

	logger.Trace().
		Str("stack", c.wd()).
		Msg("Log metadata.")
	c.log("Available metadata:")

	for _, stack := range metadata.Stacks {
		logger.Debug().
			Str("stack", c.wd()+stack.Path).
			Msg("Print metadata for individual stack.")
		c.log("\nstack %q:", stack.Path)
		c.log("\tterramate.name=%q", stack.Name)
		c.log("\tterramate.path=%q", stack.Path)
	}

	return nil
}

func (c *cli) runOnStacks() error {
	logger := log.With().
		Str("action", "runOnStacks()").
		Str("stack", c.wd()).
		Logger()

	logger.Trace().
		Msg("Create new terramate manager.")
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)

	logger.Trace().
		Msg("Get list of stacks.")
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	if c.parsedArgs.Changed {
		c.log("Running on changed stacks:")
	} else {
		c.log("Running on all stacks:")
	}

	logger.Trace().
		Msg("Filter stacks by working directory.")
	entries = c.filterStacksByWorkingDir(entries)

	logger.Trace().
		Msg("Create array of stacks.")
	stacks := make([]stack.S, len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack
	}

	logger.Trace().
		Msg("Get command to run.")
	cmdName := c.parsedArgs.Run.Command[0]
	args := c.parsedArgs.Run.Command[1:]
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	logger.Trace().
		Msg("Get order of stacks to run command on.")
	order, reason, err := terramate.RunOrder(c.root(), stacks, c.parsedArgs.Changed)
	if err != nil {
		if errors.Is(err, dag.ErrCycleDetected) {
			return fmt.Errorf("%w: reason is %s", err, reason)
		} else {
			return fmt.Errorf("failed to plan execution: %w", err)
		}
	}

	if c.parsedArgs.Run.DryRun {
		logger.Trace().
			Msg("Do a dry run - get order without actually running command.")
		if len(order) > 0 {
			c.log("The stacks will be executed using order below:")

			for i, s := range order {
				stackdir, _ := c.friendlyFmtDir(s.Dir)
				c.log("\t%d. %s (%s)", i, s.Name(), stackdir)
			}
		} else {
			c.log("No stacks will be executed.")
		}

		return nil
	}

	logger.Debug().
		Msg("Run command.")
	err = terramate.Run(c.root(), order, cmd)
	if err != nil {
		c.logerr("warn: failed to execute command: %v", err)
	}

	return nil
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
		Str("stack", c.wd()).
		Logger()

	logger.Trace().
		Msg("Get current git branch.")
	branch, err := g.CurrentBranch()
	if err != nil {
		return fmt.Errorf("checking local branch is updated: %v", err)
	}

	if branch != defaultBranch {
		return nil
	}

	c.logerr("current branch %q is the default branch, checking if it is updated.", branch)
	c.logerr("retrieving info from remote branch: %s/%s ...", defaultRemote, defaultBranch)

	logger.Trace().
		Msg("Fetch remote reference.")
	remoteRef, err := g.FetchRemoteRev(defaultRemote, defaultBranch)
	if err != nil {
		return fmt.Errorf("checking local branch %q is update: %v", branch, err)
	}
	c.logerr("retrieved info from remote branch: %s/%s.", defaultRemote, defaultBranch)

	logger.Trace().
		Msg("Get local commit ID.")
	localCommitID, err := g.RevParse(branch)
	if err != nil {
		return fmt.Errorf("checking local branch %q is update: %v", branch, err)
	}

	localRef := git.Ref{CommitID: localCommitID}

	if localRef.CommitID != remoteRef.CommitID {
		return fmt.Errorf(
			"%w: remote %s/%s=%q != local %s=%q",
			ErrOutdatedLocalRev,
			defaultRemote,
			defaultBranch,
			remoteRef.ShortCommitID(),
			branch,
			localRef.ShortCommitID(),
		)

	}

	return nil
}

func (c *cli) friendlyFmtDir(dir string) (string, bool) {
	return prj.FriendlyFmtDir(c.root(), c.wd(), dir)
}

func (c *cli) filterStacksByWorkingDir(stacks []terramate.Entry) []terramate.Entry {
	logger := log.With().
		Str("action", "filterStacksByWorkingDir()").
		Str("stack", c.wd()).
		Logger()

	logger.Trace().
		Msg("Get relative working directory.")
	relwd := prj.RelPath(c.root(), c.wd())

	logger.Trace().
		Msg("Get filtered stacks.")
	filtered := []terramate.Entry{}
	for _, e := range stacks {
		if strings.HasPrefix(e.Stack.Dir, relwd) {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

func newGit(basedir string, inheritEnv bool, checkrepo bool) (*git.Git, error) {
	log.Debug().
		Str("action", "newGit()").
		Msg("Create new git wrapper providing config.")
	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
		InheritEnv: inheritEnv,
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
		Str("stack", wd).
		Logger()

	logger.Trace().
		Msg("Create new git wrapper.")
	gw, err := newGit(wd, false, false)
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
		Str("stack", p.wd).
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
			gw, err := newGit(p.wd, false, false)
			if err != nil {
				return err
			}

			logger.Trace().
				Str("configFile", p.root+"/terramate.tm.hcl").
				Msg("Get current branch.")
			branch, err := gw.CurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current git branch: %v", err)
			}

			if branch == gitOpt.DefaultBranch {
				baseRef = gitOpt.DefaultBranchBaseRef
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

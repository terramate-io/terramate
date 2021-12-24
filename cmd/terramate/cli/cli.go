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

	"github.com/alecthomas/kong"
	"github.com/emicklei/dot"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/git"
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
	defaultRemote       = "origin"
	defaultBranch       = "main"
	defaultMainBaseRef  = "HEAD^1"
	defaultOtherBaseRef = defaultRemote + "/" + defaultBranch
)

type cliSpec struct {
	Version struct{} `cmd:"" help:"Terramate version."`

	GitChangeBase string `short:"B" default:"${baseRef}" optional:"true" help:"git base ref for computing changes."`

	Init struct {
		StackDirs []string `arg:"" name:"paths" optional:"true" help:"the stack directory (current directory if not set)."`
		Force     bool     `help:"force initialization."`
	} `cmd:"" help:"Initialize a stack."`

	List struct {
		Changed bool   `short:"c" help:"Shows only changed stacks."`
		Why     bool   `help:"Shows reason on why the stack has changed."`
		BaseDir string `arg:"" optional:"true" name:"path" type:"path" help:"base stack directory."`
	} `cmd:"" help:"List stacks."`

	Run struct {
		Quiet   bool     `short:"q" help:"Don't print any information other than the command output."`
		Changed bool     `short:"c" help:"Run on all changed stacks."`
		DryRun  bool     `default:"false" help:"plan the execution but do not execute it"`
		Basedir string   `short:"b" optional:"true" help:"Run on stacks inside basedir."`
		Command []string `arg:"" name:"cmd" passthrough:"" help:"command to execute."`
	} `cmd:"" help:"Run command in the stacks."`

	Plan struct {
		Graph struct {
			Outfile string `short:"o" default:"" help:"output .dot file."`
			Label   string `short:"l" default:"stack.name" help:"Label used in graph nodes (it could be either \"stack.name\" or \"stack.dir\"."`
			Basedir string `arg:"" optional:"true" help:"base directory to search stacks."`
		} `cmd:"" help:"generate a graph of the execution order."`

		RunOrder struct {
			Basedir string `arg:"" optional:"true" help:"base directory to search stacks."`
			Changed bool   `short:"c" help:"Shows run order of changed stacks."`
		} `cmd:"" help:"show the topological ordering of the stacks"`
	} `cmd:"" help:"plan execution."`
	Generate struct {
		Basedir string `short:"b" optional:"true" help:"Generate code for stacks inside basedir."`
	} `cmd:"" help:"Generate terraform code for stacks."`

	Metadata struct {
	} `cmd:"" help:"shows metadata available on the project"`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
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
// Each Run call is completely isolated from each other (no shared state)
// as far as the parameters are not shared between the Run calls.
//
// If a critical error is found an non-nil error is returned.
func Run(
	wd string,
	args []string,
	inheritEnv bool,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if err := configureLogging(stderr); err != nil {
		return err
	}

	c, err := newCLI(wd, args, inheritEnv, stdin, stdout, stderr)
	if err != nil {
		return err
	}
	return c.run()
}

type cli struct {
	ctx        *kong.Context
	parsedArgs *cliSpec
	inheritEnv bool
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	exit       bool
	wd         string
	baseRef    string
}

func newCLI(
	wd string,
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

	parsedArgs := cliSpec{}
	kongExit := false
	kongExitStatus := 0

	gw, err := newGit(wd, inheritEnv, false)
	if err != nil {
		return nil, err
	}

	baseRef := defaultOtherBaseRef
	if gw.IsRepository() {
		branch, err := gw.CurrentBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to get current git branch: %v", err)
		}

		if branch == "main" {
			baseRef = defaultMainBaseRef
		}
	}

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
		kong.Vars{
			"baseRef": baseRef,
		},
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

	return &cli{
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		inheritEnv: inheritEnv,
		parsedArgs: &parsedArgs,
		ctx:        ctx,
		baseRef:    parsedArgs.GitChangeBase,
		wd:         wd,
	}, nil
}

func (c *cli) run() error {
	if c.exit {
		// WHY: parser called exit but with no error (like help)
		return nil
	}

	switch c.ctx.Command() {
	case "version":
		c.log(terramate.Version())
	case "init":
		return c.initStack([]string{c.wd})
	case "init <paths>":
		return c.initStack(c.parsedArgs.Init.StackDirs)
	case "list":
		return c.printStacks(c.wd)
	case "list <path>":
		return c.printStacks(c.parsedArgs.List.BaseDir)
	case "plan graph":
		fallthrough
	case "plan graph <basedir>":
		basedir := c.wd
		if c.parsedArgs.Plan.Graph.Basedir != "" {
			basedir = strings.TrimSuffix(c.parsedArgs.Plan.Graph.Basedir, "/")
		}
		return c.generateGraph(basedir)
	case "plan run-order":
		fallthrough
	case "plan run-order <basedir>":
		basedir := c.wd
		if c.parsedArgs.Plan.RunOrder.Basedir != "" {
			basedir = strings.TrimSuffix(c.parsedArgs.Plan.RunOrder.Basedir, "/")
		}
		return c.printRunOrder(basedir)
	case "run":
		if len(c.parsedArgs.Run.Command) == 0 {
			return errors.New("no command specified")
		}
		fallthrough
	case "run <cmd>":
		basedir := c.wd
		if c.parsedArgs.Run.Basedir != "" {
			basedir = strings.TrimSuffix(c.parsedArgs.Run.Basedir, "/")
		}
		return c.runOnStacks(basedir)
	case "generate":
		return terramate.Generate(c.wd)
	case "metadata":
		return c.printMetadata()
	case "install-completions":
		return c.parsedArgs.InstallCompletions.Run(c.ctx)
	default:
		return fmt.Errorf("unexpected command sequence: %s", c.ctx.Command())
	}

	return nil
}

func (c *cli) initStack(dirs []string) error {
	var errmsgs []string
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(c.wd, d)
		}

		err := terramate.Init(d, c.parsedArgs.Init.Force)
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

func (c *cli) listStacks(
	basedir string,
	mgr *terramate.Manager,
	isChanged bool,
) ([]terramate.Entry, error) {

	if isChanged {
		git, err := newGit(basedir, c.inheritEnv, true)
		if err != nil {
			return nil, err
		}
		if err := c.checkDefaultRemote(git); err != nil {
			return nil, err
		}
		if err := c.checkLocalDefaultIsUpdated(git); err != nil {
			return nil, err
		}
		return mgr.ListChanged()
	}

	return mgr.List()
}

func (c *cli) printStacks(basedir string) error {
	mgr := terramate.NewManager(basedir, c.baseRef)
	entries, err := c.listStacks(basedir, mgr, c.parsedArgs.List.Changed)
	if err != nil {
		return err
	}

	trimPart := c.wd + string(os.PathSeparator)
	for _, entry := range entries {
		stack := entry.Stack
		stackdir := strings.TrimPrefix(stack.Dir, trimPart)

		if c.parsedArgs.List.Why {
			c.log("%s - %s", stackdir, entry.Reason)
		} else {
			c.log(stackdir)
		}
	}
	return nil
}

func (c *cli) generateGraph(basedir string) error {
	var getLabel func(s stack.S) string

	switch c.parsedArgs.Plan.Graph.Label {
	case "stack.name":
		getLabel = func(s stack.S) string { return s.Name() }
	case "stack.dir":
		getLabel = func(s stack.S) string { return s.Dir }
	default:
		return fmt.Errorf("-label expects the values \"stack.name\" or \"stack.dir\"")
	}
	entries, err := terramate.ListStacks(basedir)
	if err != nil {
		return err
	}

	loader := stack.NewLoader()

	di := dot.NewGraph(dot.Directed)

	for _, e := range entries {
		tree, err := terramate.BuildOrderTree(e.Stack, loader)
		if err != nil {
			return fmt.Errorf("failed to build order tree: %w", err)
		}

		node := di.Node(getLabel(tree.Stack))
		generateDot(di, node, tree, getLabel)
	}

	outFile := c.parsedArgs.Plan.Graph.Outfile
	var out io.Writer
	if outFile == "" {
		out = c.stdout
	} else {
		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("opening file %q: %w", outFile, err)
		}

		defer f.Close()

		out = f
	}

	_, err = out.Write([]byte(di.String()))
	if err != nil {
		return fmt.Errorf("writing output to %q: %w", outFile, err)
	}

	return nil
}

func generateDot(
	g *dot.Graph,
	parent dot.Node,
	tree terramate.OrderDAG,
	getLabel func(s stack.S) string,
) {
	if tree.Cycle {
		return
	}

	for _, s := range tree.Order {
		n := g.Node(getLabel(s.Stack))

		edges := g.FindEdges(parent, n)
		if len(edges) == 0 {
			edge := g.Edge(parent, n)
			if s.Cycle {
				edge.Attr("color", "red")
			}
		}

		if s.Cycle {
			continue
		}

		generateDot(g, n, s, getLabel)
	}
}

func (c *cli) printRunOrder(basedir string) error {
	if !filepath.IsAbs(basedir) {
		basedir = filepath.Join(c.wd, basedir)
	}

	mgr := terramate.NewManager(basedir, c.baseRef)
	entries, err := c.listStacks(basedir, mgr, c.parsedArgs.Run.Changed)
	if err != nil {
		return err
	}

	stacks := make([]stack.S, len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack
	}

	order, err := terramate.RunOrder(stacks, c.parsedArgs.Plan.RunOrder.Changed)
	if err != nil {
		c.logerr("error: %v", err)
		return err
	}

	for _, s := range order {
		c.log("%s", s)
	}

	return nil
}

func (c *cli) printMetadata() error {
	metadata, err := terramate.LoadMetadata(c.wd)
	if err != nil {
		return err
	}

	c.log("Available metadata:")

	for _, stack := range metadata.Stacks {
		c.log("\nstack %q:", stack.Path)
		c.log("\tterraform.name=%q", stack.Name)
		c.log("\tterraform.path=%q", stack.Path)
	}

	return nil
}

func (c *cli) runOnStacks(basedir string) error {
	if !filepath.IsAbs(basedir) {
		basedir = filepath.Join(c.wd, basedir)
	}

	mgr := terramate.NewManager(basedir, c.baseRef)
	entries, err := c.listStacks(basedir, mgr, c.parsedArgs.Run.Changed)
	if err != nil {
		return err
	}

	if c.parsedArgs.Run.Changed {
		c.log("Running on changed stacks:")
	} else {
		c.log("Running on all stacks:")
	}

	stacks := make([]stack.S, len(entries))
	for i, e := range entries {
		stacks[i] = e.Stack
	}

	cmdName := c.parsedArgs.Run.Command[0]
	args := c.parsedArgs.Run.Command[1:]
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	order, err := terramate.RunOrder(stacks, c.parsedArgs.Run.Changed)
	if err != nil {
		return fmt.Errorf("failed to plan execution: %w", err)
	}

	if c.parsedArgs.Run.DryRun {
		if len(order) > 0 {
			c.log("The stacks will be executed using order below:")

			trimPart := c.wd + string(os.PathSeparator)

			for i, s := range order {
				c.log("\t%d. %s (%s)", i, s.Name(), strings.TrimPrefix(s.Dir, trimPart))
			}
		} else {
			c.log("No stacks will be executed.")
		}

		return nil
	}

	err = terramate.Run(order, cmd)
	if err != nil {
		c.logerr("warn: failed to execute command: %v", err)
	}

	return nil
}

func (c *cli) log(format string, args ...interface{}) {
	fmt.Fprintln(c.stdout, fmt.Sprintf(format, args...))
}

func (c *cli) logerr(format string, args ...interface{}) {
	fmt.Fprintln(c.stderr, fmt.Sprintf(format, args...))
}

func (c *cli) checkDefaultRemote(g *git.Git) error {
	remotes, err := g.Remotes()
	if err != nil {
		return fmt.Errorf("checking if remote %q exists: %v", defaultRemote, err)
	}

	var defRemote *git.Remote

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
	branch, err := g.CurrentBranch()
	if err != nil {
		return fmt.Errorf("checking local branch is updated: %v", err)
	}

	if branch != defaultBranch {
		return nil
	}

	c.logerr("current branch %q is the default branch, checking if it is updated.", branch)
	c.logerr("retrieving info from remote branch: %s/%s ...", defaultRemote, defaultBranch)

	remoteRef, err := g.FetchRemoteRev(defaultRemote, defaultBranch)
	if err != nil {
		return fmt.Errorf("checking local branch %q is update: %v", branch, err)
	}
	c.logerr("retrieved info from remote branch: %s/%s.", defaultRemote, defaultBranch)

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

func newGit(basedir string, inheritEnv bool, checkrepo bool) (*git.Git, error) {
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

func configureLogging(output io.Writer) error {
	level := lookupEnv("TM_LOG", "INFO")
	fmt := lookupEnv("TM_LOG_FMT", "CONSOLE")

	zloglevel, err := getzlogLevel(level)
	if err != nil {
		return err
	}

	logwriter, err := getzlogWriter(fmt, output)
	if err != nil {
		return err
	}

	zerolog.SetGlobalLevel(zloglevel)
	log.Logger = zerolog.New(logwriter).With().Timestamp().Logger()
	return nil
}

func lookupEnv(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func getzlogLevel(level string) (zerolog.Level, error) {
	switch level {
	case "DEBUG":
		return zerolog.DebugLevel, nil
	case "INFO":
		return zerolog.InfoLevel, nil
	case "WARN":
		return zerolog.WarnLevel, nil
	case "ERROR":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.NoLevel, fmt.Errorf("unknown log level %q", level)
	}
}

func getzlogWriter(format string, output io.Writer) (io.Writer, error) {
	switch format {
	case "CONSOLE":
		return zerolog.ConsoleWriter{Out: output}, nil
	case "JSON":
		// Default is JSON on zlog
		return output, nil
	default:
		return nil, fmt.Errorf("unknown log format %q", format)
	}
}

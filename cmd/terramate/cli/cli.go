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
	defaultBranchBaseRef = "HEAD^1"
)

type cliSpec struct {
	Version       struct{} `cmd:"" help:"Terramate version."`
	Chdir         string   `short:"C" optional:"true" help:"sets working directory."`
	GitChangeBase string   `short:"B" optional:"true" help:"git base ref for computing changes."`
	Changed       bool     `short:"c" optional:"true" help:"filter by changed infrastructure"`

	Init struct {
		StackDirs []string `arg:"" name:"paths" optional:"true" help:"the stack directory (current directory if not set)."`
		Force     bool     `help:"force initialization."`
	} `cmd:"" help:"Initialize a stack."`

	List struct {
		Why bool `help:"Shows the reason why the stack has changed."`
	} `cmd:"" help:"List stacks."`

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

	Generate struct{} `cmd:"" help:"Generate terraform code for stacks."`
	Metadata struct{} `cmd:"" help:"shows metadata available on the project"`

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
	args []string,
	inheritEnv bool,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	c, err := newCLI(args, inheritEnv, stdin, stdout, stderr)
	if err != nil {
		return err
	}
	return c.run()
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

func newCLI(
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

	wd := parsedArgs.Chdir
	if wd == "" {
		wd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return nil, fmt.Errorf("failed evaluating symlinks for %q: %w", wd, err)
	}

	wd, err = filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of %q: %w", wd, err)
	}

	err = os.Chdir(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to change working directory to %q: %w", wd, err)
	}

	prj, foundRoot, err := lookupProject(wd)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup project root from %q: %w", wd, err)
	}

	if !foundRoot {
		return nil, fmt.Errorf("project root not found")
	}

	err = prj.setDefaults(&parsedArgs)
	if err != nil {
		return nil, fmt.Errorf("setting configuration: %w", err)
	}

	if parsedArgs.Changed && !prj.isRepo {
		return nil, fmt.Errorf("flag --changed provided but no git repository found.")
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

func (c *cli) run() error {
	if c.exit {
		// WHY: parser called exit but with no error (like help)
		return nil
	}

	if c.parsedArgs.Changed {
		git, err := newGit(c.root(), c.inheritEnv, true)
		if err != nil {
			return err
		}
		if err := c.checkDefaultRemote(git); err != nil {
			return err
		}
		if err := c.checkLocalDefaultIsUpdated(git); err != nil {
			return err
		}
	}

	switch c.ctx.Command() {
	case "version":
		c.log(terramate.Version())
	case "init":
		return c.initStack([]string{c.wd()})
	case "init <paths>":
		return c.initStack(c.parsedArgs.Init.StackDirs)
	case "list":
		return c.printStacks()
	case "plan graph":
		return c.generateGraph()
	case "plan run-order":
		return c.printRunOrder()
	case "run":
		if len(c.parsedArgs.Run.Command) == 0 {
			return errors.New("no command specified")
		}
		fallthrough
	case "run <cmd>":
		return c.runOnStacks()
	case "generate":
		return terramate.Generate(c.wd())
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
			d = filepath.Join(c.wd(), d)
		}

		err := terramate.Init(c.root(), d, c.parsedArgs.Init.Force)
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
		return mgr.ListChanged()
	}

	return mgr.List()
}

func (c *cli) printStacks() error {
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		stack := entry.Stack
		stackdir, ok := c.showdir(stack.Dir)
		if !ok {
			continue
		}

		if c.parsedArgs.List.Why {
			c.log("%s - %s", stackdir, entry.Reason)
		} else {
			c.log(stackdir)
		}
	}
	return nil
}

func (c *cli) generateGraph() error {
	var getLabel func(s stack.S) string

	switch c.parsedArgs.Plan.Graph.Label {
	case "stack.name":
		getLabel = func(s stack.S) string { return s.Name() }
	case "stack.dir":
		getLabel = func(s stack.S) string { return s.Dir }
	default:
		return fmt.Errorf("-label expects the values \"stack.name\" or \"stack.dir\"")
	}
	entries, err := terramate.ListStacks(c.root())
	if err != nil {
		return err
	}

	loader := stack.NewLoader(c.root())
	di := dot.NewGraph(dot.Directed)

	relwd := prj.RelPath(c.root(), c.wd())
	for _, e := range entries {
		if !strings.HasPrefix(e.Stack.Dir, relwd) {
			continue
		}

		tree, err := terramate.BuildOrderTree(c.root(), e.Stack, loader)
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

func (c *cli) printRunOrder() error {
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	relwd := prj.RelPath(c.root(), c.wd())
	stacks := make([]stack.S, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Stack.Dir, relwd) {
			stacks = append(stacks, e.Stack)
		}
	}

	order, err := terramate.RunOrder(c.root(), stacks, c.parsedArgs.Changed)
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
	metadata, err := terramate.LoadMetadata(c.wd())
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

func (c *cli) runOnStacks() error {
	mgr := terramate.NewManager(c.root(), c.prj.baseRef)
	entries, err := c.listStacks(mgr, c.parsedArgs.Changed)
	if err != nil {
		return err
	}

	if c.parsedArgs.Changed {
		c.log("Running on changed stacks:")
	} else {
		c.log("Running on all stacks:")
	}

	relwd := prj.RelPath(c.root(), c.wd())
	stacks := make([]stack.S, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Stack.Dir, relwd) {
			stacks = append(stacks, e.Stack)
		}
	}

	cmdName := c.parsedArgs.Run.Command[0]
	args := c.parsedArgs.Run.Command[1:]
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	order, err := terramate.RunOrder(c.root(), stacks, c.parsedArgs.Changed)
	if err != nil {
		return fmt.Errorf("failed to plan execution: %w", err)
	}

	if c.parsedArgs.Run.DryRun {
		if len(order) > 0 {
			c.log("The stacks will be executed using order below:")

			for i, s := range order {
				stackdir, _ := c.showdir(s.Dir)
				c.log("\t%d. %s (%s)", i, s.Name(), stackdir)
			}
		} else {
			c.log("No stacks will be executed.")
		}

		return nil
	}

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

func (c *cli) showdir(dir string) (string, bool) {
	return prj.ShowDir(c.root(), c.wd(), dir)
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

func lookupProject(wd string) (prj project, found bool, err error) {
	prj = project{
		wd: wd,
	}
	gw, err := newGit(wd, false, false)
	if err == nil {
		gitdir, err := gw.Root()
		if err == nil {
			gitabs, err := filepath.Abs(gitdir)
			if err != nil {
				return project{}, false, fmt.Errorf("getting absolute path of %q: %w", gitdir, err)
			}

			gitabs, err = filepath.EvalSymlinks(gitabs)
			if err != nil {
				return project{}, false, fmt.Errorf("failed evaluating symlinks of %q: %w",
					gitabs, err)
			}

			root := filepath.Dir(gitabs)
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
	if p.rootcfg.Terramate == nil {
		// if config has no terramate block we create one with default
		// configurations.
		p.rootcfg.Terramate = &hcl.Terramate{}
	}

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
			gw, err := newGit(p.wd, false, false)
			if err != nil {
				return err
			}

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

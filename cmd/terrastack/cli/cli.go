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
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/git"
)

const (
	ErrOutdatedLocalRev      errutil.Error = "outdated local revision"
	ErrNoDefaultRemoteConfig errutil.Error = "repository must have a configured origin/main"
)

const (
	defaultRemote       = "origin"
	defaultBranch       = "main"
	defaultMainBaseRef  = "HEAD^1"
	defaultOtherBaseRef = defaultRemote + "/" + defaultBranch
)

type cliSpec struct {
	Version struct{} `cmd:"" help:"Terrastack version."`

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
		Basedir string   `short:"b" optional:"true" help:"Run on stacks inside basedir."`
		Command []string `arg:"" name:"cmd" passthrough:"" help:"command to execute."`
	} `cmd:"" help:"Run command in the stacks."`
}

// Run will run terrastack with the provided flags defined on args from the
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
func Run(wd string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	c, err := newCLI(wd, args, stdin, stdout, stderr)
	if err != nil {
		return err
	}
	return c.run()
}

type cli struct {
	ctx        *kong.Context
	parsedArgs *cliSpec
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	exit       bool
	wd         string
	baseRef    string
}

func newCLI(wd string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (*cli, error) {
	if len(args) == 0 {
		// WHY: avoid default kong error, print help
		args = []string{"--help"}
	}

	parsedArgs := cliSpec{}
	kongExit := false
	kongExitStatus := 0

	gw, err := git.WithConfig(git.Config{
		WorkingDir: wd,
	})
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
		kong.Name("terrastack"),
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
		c.log(terrastack.Version())
	case "init":
		return c.initStack([]string{c.wd})
	case "init <paths>":
		return c.initStack(c.parsedArgs.Init.StackDirs)
	case "list":
		return c.printStacks(c.wd)
	case "list <path>":
		return c.printStacks(c.parsedArgs.List.BaseDir)
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

	default:
		return fmt.Errorf("unexpected command sequence: %s", c.ctx.Command())
	}

	return nil
}

func (c *cli) initStack(dirs []string) error {
	var nErrors int
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			d = filepath.Join(c.wd, d)
		}

		err := terrastack.Init(d, c.parsedArgs.Init.Force)
		if err != nil {
			c.logerr("warn: failed to initialize stack: %v", err)
			nErrors++
		}
	}

	if nErrors > 0 {
		return fmt.Errorf("failed to initialize %d stack(s)", nErrors)
	}
	return nil
}

func (c *cli) listStacks(
	basedir string,
	mgr *terrastack.Manager,
	isChanged bool,
) ([]terrastack.Entry, error) {

	if isChanged {
		git, err := newGit(basedir)
		if err != nil {
			return nil, err
		}
		if err := c.checkDefaultRemote(git); err != nil {
			return nil, err
		}
		if err := c.checkDefaultBranch(git); err != nil {
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
	mgr := terrastack.NewManager(basedir, c.baseRef)
	stacks, err := c.listStacks(basedir, mgr, c.parsedArgs.List.Changed)
	if err != nil {
		return err
	}

	trimPart := c.wd + string(os.PathSeparator)
	for _, stack := range stacks {
		stackdir := strings.TrimPrefix(stack.Dir, trimPart)

		if c.parsedArgs.List.Why {
			c.log("%s - %s", stackdir, stack.Reason)
		} else {
			c.log(stackdir)
		}
	}
	return nil
}

func (c *cli) runOnStacks(basedir string) error {
	var nErrors int

	if !filepath.IsAbs(basedir) {
		basedir = filepath.Join(c.wd, basedir)
	}

	mgr := terrastack.NewManager(basedir, c.baseRef)
	stacks, err := c.listStacks(basedir, mgr, c.parsedArgs.Run.Changed)
	if err != nil {
		return err
	}

	if c.parsedArgs.Run.Changed {
		c.log("Running on changed stacks:")
	} else {
		c.log("Running on all stacks:")
	}

	cmdName := c.parsedArgs.Run.Command[0]
	args := c.parsedArgs.Run.Command[1:]

	for _, stack := range stacks {
		cmd := exec.Command(cmdName, args...)
		cmd.Dir = stack.Dir
		cmd.Stdin = c.stdin
		cmd.Stdout = c.stdout
		cmd.Stderr = c.stderr

		c.log("[%s] running %s", stack.Dir, cmd)

		err = cmd.Run()
		if err != nil {
			c.logerr("warn: failed to execute command: %v", err)
			nErrors++
		}
	}

	if nErrors != 0 {
		return fmt.Errorf("some (%d) commands failed", nErrors)
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
		return fmt.Errorf("checking remote %q exists: %v", defaultRemote, err)
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

func (c *cli) checkDefaultBranch(g *git.Git) error {
	//TODO(katcipis): implement =P
	return nil
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

func newGit(basedir string) (*git.Git, error) {
	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
	})

	if err != nil {
		return nil, err
	}

	if !g.IsRepository() {
		return nil, fmt.Errorf("dir %q is not a git repository", basedir)
	}

	return g, nil
}

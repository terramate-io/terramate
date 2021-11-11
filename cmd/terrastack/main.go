package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/mineiros-io/terrastack"
)

var cliSpec struct {
	Version struct{} `cmd:"" help:"Terrastack version."`

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

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error: failed to get current directory: %v", err)
	}

	ctx := kong.Parse(&cliSpec,
		kong.Name("terrastack"),
		kong.Description("A tool for managing terraform stacks"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}))

	switch ctx.Command() {
	case "version":
		fmt.Println(terrastack.Version())
	case "init":
		initStack(wd, []string{wd})
	case "init <paths>":
		initStack(wd, cliSpec.Init.StackDirs)
	case "list":
		printStacks(wd, wd)
	case "list <path>":
		printStacks(cliSpec.List.BaseDir, wd)
	case "run":
		if len(cliSpec.Run.Command) == 0 {
			log.Fatalf("no command specified")
		}

		fallthrough
	case "run <cmd>":
		basedir := wd
		if cliSpec.Run.Basedir != "" {
			basedir = strings.TrimSuffix(cliSpec.Run.Basedir, "/")
		}

		run(basedir)

	default:
		log.Fatalf("unexpected command sequence: %s", ctx.Command())
	}
}

func initStack(root string, dirs []string) {
	var nErrors int
	mgr := terrastack.NewManager(root)
	for _, d := range dirs {
		err := mgr.Init(d, cliSpec.Init.Force)
		if err != nil {
			log.Printf("warn: failed to initialize stack: %v", err)
			nErrors++
		}
	}

	if nErrors > 0 {
		log.Fatalf("failed to initialize %d stack(s)", nErrors)
	}
}

func listStacks(mgr *terrastack.Manager) ([]terrastack.Entry, error) {
	var (
		err    error
		stacks []terrastack.Entry
	)

	if cliSpec.List.Changed {
		stacks, err = mgr.ListChanged()
	} else {
		stacks, err = mgr.List()
	}

	return stacks, err
}

func printStacks(basedir string, cwd string) {
	mgr := terrastack.NewManager(basedir)
	stacks, err := listStacks(mgr)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	cwd = cwd + string(os.PathSeparator)

	for _, stack := range stacks {
		stackdir := strings.TrimPrefix(stack.Dir, cwd)

		fmt.Print(stackdir)

		if cliSpec.List.Why {
			fmt.Printf(" - %s", stack.Reason)
		}

		fmt.Printf("\n")
	}
}

func run(basedir string) {
	var nErrors int

	basedir, err := filepath.Abs(basedir)
	if err != nil {
		log.Fatalf("error computing absolute path: %v", err)
	}

	mgr := terrastack.NewManager(basedir)
	stacks, err := listStacks(mgr)
	if err != nil {
		log.Fatalf("error: failed to list stacks: %v", err)
	}

	if cliSpec.Run.Changed {
		printf("Running on changed stacks:\n")
	} else {
		printf("Running on all stacks:\n")
	}

	cmdName := cliSpec.Run.Command[0]
	args := cliSpec.Run.Command[1:]

	for _, stack := range stacks {

		cmd := exec.Command(cmdName, args...)
		cmd.Dir = stack.Dir

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		cmd.Env = os.Environ()

		printf("[%s] running %s\n", stack.Dir, cmd)

		err = cmd.Run()
		if err != nil {
			log.Printf("warn: failed to execute command: %v", err)
			nErrors++
		}

		printf("\n")
	}

	if nErrors != 0 {
		log.Fatalf("warn: some (%d) commands failed", nErrors)
	}
}

func printf(format string, args ...interface{}) {
	if !cliSpec.Run.Quiet {
		fmt.Printf(format, args...)
	}
}

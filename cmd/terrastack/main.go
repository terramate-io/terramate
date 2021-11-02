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
	Version struct{} `cmd:"" help:"Terrastack version"`

	Init struct {
		StackDirs []string `arg:"" name:"paths" optional:"true" help:"the stack directory (current directory if not set)"`
		Force     bool     `help:"force initialization"`
	} `cmd:"" help:"Initialize a stack"`

	List struct {
		Changed bool   `short:"c" help:"Shows only changed stacks"`
		BaseDir string `arg:"" optional:"true" name:"path" type:"path" help:"base stack directory"`
	} `cmd:"" help:"List stacks."`

	Run struct {
		Quiet   bool     `short:"q" help:"Don't print any information other than the command output."`
		Changed bool     `short:"c" help:"Run on all changed stacks"`
		Basedir string   `short:"b" optional:"true" help:"Run on stacks inside basedir"`
		Command []string `arg:"" name:"cmd" passthrough:"" help:"command to execute"`
	} `cmd:"" help:"Run command in the stacks"`
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
		initStack([]string{wd})
	case "init <paths>":
		initStack(cliSpec.Init.StackDirs)
	case "list":
		listStacks(wd, wd)
	case "list <path>":
		listStacks(cliSpec.List.BaseDir, wd)
	case "run":
		if len(cliSpec.Run.Command) == 0 {
			log.Fatalf("no command specified")
		}

		fallthrough
	case "run <cmd>":
		run(wd)
	default:
		log.Fatalf("unexpected command sequence: %s", ctx.Command())
	}
}

func initStack(dirs []string) {
	var nErrors int
	for _, d := range dirs {
		err := terrastack.Init(d, cliSpec.Init.Force)
		if err != nil {
			log.Printf("warn: failed to initialize stack: %v", err)
			nErrors++
		}
	}

	if nErrors > 0 {
		log.Fatalf("failed to initialize %d stack(s)", nErrors)
	}
}

func listStacks(basedir string, cwd string) {
	var (
		stacks []string
		err    error
	)

	if cliSpec.List.Changed {
		stacks, err = terrastack.ListChanged(basedir)
	} else {
		stacks, err = terrastack.List(basedir)
	}

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	cwd = cwd + string(os.PathSeparator)

	for _, stack := range stacks {
		stack = strings.TrimPrefix(stack, cwd)

		fmt.Println(stack)
	}
}

func run(dir string) {
	var (
		stacks  []string
		err     error
		nErrors int
	)

	if !cliSpec.Run.Changed {
		printf("Running on all stacks:\n")
		stacks, err = terrastack.List(dir)
	} else {
		printf("Running on changed stacks:\n")
		stacks, err = terrastack.ListChanged(dir)
	}

	if err != nil {
		log.Fatalf("error: failed to list stacks: %v", err)
	}

	cmdName := cliSpec.Run.Command[0]
	args := cliSpec.Run.Command[1:]

	basedir := cliSpec.Run.Basedir

	if basedir != "" {
		basedir, err = filepath.Abs(basedir)
		if err != nil {
			log.Fatalf("error computing absolute path: %v", err)
		}
	}

	for _, stack := range stacks {
		if !strings.HasPrefix(stack, basedir) {
			continue
		}

		stack = strings.TrimPrefix(stack, basedir)

		printf("[%s] running %s %s\n", stack, cmdName, strings.Join(args, " "))

		cmd := exec.Command(cmdName, args...)
		cmd.Dir = stack

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		cmd.Env = os.Environ()

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

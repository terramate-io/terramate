// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// helper is a utility command that implements behaviors that are
// useful when testing terramate run features in a way that reduces
// dependencies on the environment to run the tests.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/tfjson"
	"github.com/terramate-io/tfjson/sanitize"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s requires at least one subcommand argument", os.Args[0])
	}

	execName := filepath.Base(os.Args[0])
	switch execName {
	case "terragrunt", "terragrunt.exe":
		terragrunt(os.Args[1:]...)
		return
	}

	// note: unrecovered panic() aborts the program with exit code 2 and this
	// could be confused with a *detected drift* (see: run --sync-drift-status)
	// then avoid panics here and do proper os.Exit(1) in case of errors.

	switch os.Args[1] {
	case "echo":
		args := os.Args[2:]
		for i, arg := range args {
			fmt.Print(arg)
			if i+1 < len(args) {
				fmt.Print(" ")
			}
		}
		fmt.Print("\n")
	case "true":
		os.Exit(0)
	case "false":
		os.Exit(1)
	case "exit":
		exit(os.Args[2])
	case "hang":
		hang()
	case "sleep":
		sleep(os.Args[2])
	case "env":
		env(os.Args[2], os.Args[3:]...)
	case "env-prefix":
		envPrefix(os.Args[2], os.Args[3])
	case "cat":
		cat(os.Args[2])
	case "rm":
		rm(os.Args[2])
	case "tempdir":
		tempDir()
	case "stack-abs-path":
		stackAbsPath(os.Args[2])
	case "stack-rel-path":
		stackRelPath(os.Args[2])
	case "tf-plan-sanitize":
		tfPlanSanitize(os.Args[2])
	case "fibonacci":
		fibonacci()
	case "git-normalization":
		gitnorm(os.Args[2])
	case "terragrunt":
		terragrunt(os.Args[2:]...)
	case "prompt":
		prompt()
	default:
		log.Fatalf("unknown command %s", os.Args[1])
	}
}

// hang will hang the process forever, ignoring any signals.
// It is useful to validate forced kill behavior.
// It will print "ready" when it starts to receive the signals.
// It will print the name of the received signals, which may also be useful in testing.
func hang() {
	signals := make(chan os.Signal, 10)
	signal.Notify(signals)

	fmt.Println("ready")

	for s := range signals {
		fmt.Println(s)
	}
}

// sleep put the test process to sleep.
func sleep(durationStr string) {
	d, err := time.ParseDuration(durationStr)
	checkerr(err)
	fmt.Println("ready")
	time.Sleep(d)
}

// exit with the provided exitCode.
func exit(exitCodeStr string) {
	code, err := strconv.Atoi(exitCodeStr)
	checkerr(err)
	os.Exit(code)
}

// env sends os.Environ() on stdout and exits.
func env(rootdir string, names ...string) {
	if len(names) > 0 {
		cwd, err := os.Getwd()
		checkerr(err)
		dir := project.PrjAbsPath(rootdir, cwd)

		for _, env := range os.Environ() {
			parts := strings.Split(env, "=")
			for _, n := range names {
				if parts[0] == n {
					fmt.Printf("%s: %s\n", dir, strings.Join(parts[1:], "="))
				}
			}
		}
		return
	}
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
}

func envPrefix(rootdir string, prefix string) {
	cwd, err := os.Getwd()
	checkerr(err)
	dir := project.PrjAbsPath(rootdir, cwd)
	for _, env := range os.Environ() {
		parts := strings.Split(env, "=")
		if strings.HasPrefix(parts[0], prefix) {
			fmt.Printf("%s: %s\n", dir, env)
		}
	}
}

// cat the file contents to stdout.
func cat(fname string) {
	bytes, err := os.ReadFile(fname)
	checkerr(err)
	fmt.Printf("%s", string(bytes))
}

// rm remove the given path.
func rm(fname string) {
	err := os.RemoveAll(fname)
	checkerr(err)
}

// tempdir creates a temporary directory.
func tempDir() {
	tmpdir, err := os.MkdirTemp("", "tm-tmpdir")
	checkerr(err)
	fmt.Print(tmpdir)
}

// fibonacci, when called from dir fib.N/, writes the Nth fibonacci number to ./fib.txt.
// It may try to read values from ../fib.N-1/fib.txt and ../fib.N-2/fib.txt, which were previously
// created by running this command in other dirs.
func fibonacci() {
	wd, err := os.Getwd()
	checkerr(err)
	dirname := filepath.Base(wd)

	if !strings.HasPrefix(dirname, "fib.") {
		log.Fatalf("fibonacci must be called from dir 'fib.N, was %v'", wd)
	}

	n, err := strconv.ParseInt(dirname[len("fib."):], 10, 64)
	checkerr(err)

	var v int64

	switch n {
	case 0:
		v = 0
	case 1:
		v = 1
	default:
		v = 0
		for _, i := range []int64{n - 1, n - 2} {
			b, err := os.ReadFile(fmt.Sprintf("../fib.%v/fib.txt", i))
			checkerr(err)
			ni, err := strconv.ParseInt(string(b), 10, 64)
			checkerr(err)
			v += ni
		}
	}

	checkerr(os.WriteFile("fib.txt", []byte(fmt.Sprintf("%v", v)), 0644))
}

func stackAbsPath(base string) {
	cwd, err := os.Getwd()
	checkerr(err)
	rel, err := filepath.Rel(base, cwd)
	checkerr(err)
	fmt.Println("/" + filepath.ToSlash(rel))
}

func stackRelPath(base string) {
	cwd, err := os.Getwd()
	checkerr(err)
	rel, err := filepath.Rel(base, cwd)
	checkerr(err)
	fmt.Println(filepath.ToSlash(rel))
}

func tfPlanSanitize(fname string) {
	var oldPlan tfjson.Plan
	oldPlanData, err := os.ReadFile(fname)
	checkerr(err)
	err = json.Unmarshal(oldPlanData, &oldPlan)
	checkerr(err)
	newPlan, err := sanitize.SanitizePlan(&oldPlan)
	checkerr(err)
	newPlanData, err := json.Marshal(newPlan)
	checkerr(err)
	fmt.Print(string(newPlanData))
}

func gitnorm(rawURL string) {
	repo, err := git.NormalizeGitURI(rawURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("host:  %s\n", repo.Host)
	fmt.Printf("owner: %s\n", repo.Owner)
	fmt.Printf("name:  %s\n", repo.Name)
	fmt.Printf("normalized repository: %s\n", repo.Repo)
}

func prompt() {
	_, _ = os.Stdout.WriteString("are you sure?\nprompt: ")
	r := bufio.NewReader(os.Stdin)
	text, _ := r.ReadString('\n')
	_, _ = os.Stdout.WriteString("\nyou entered: " + text)
}

func checkerr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

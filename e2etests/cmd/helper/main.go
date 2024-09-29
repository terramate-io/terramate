// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// helper is a utility command that implements behaviors that are
// useful when testing terramate run features in a way that reduces
// dependencies on the environment to run the tests.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	stdos "os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/os"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/tfjson"
	"github.com/terramate-io/tfjson/sanitize"
)

func main() {
	if len(stdos.Args) < 2 {
		log.Fatalf("%s requires at least one subcommand argument", stdos.Args[0])
	}

	// note: unrecovered panic() aborts the program with exit code 2 and this
	// could be confused with a *detected drift* (see: run --sync-drift-status)
	// then avoid panics here and do proper os.Exit(1) in case of errors.

	switch stdos.Args[1] {
	case "echo":
		args := stdos.Args[2:]
		for i, arg := range args {
			fmt.Print(arg)
			if i+1 < len(args) {
				fmt.Print(" ")
			}
		}
		fmt.Print("\n")
	case "true":
		stdos.Exit(0)
	case "false":
		stdos.Exit(1)
	case "exit":
		exit(stdos.Args[2])
	case "hang":
		hang()
	case "sleep":
		sleep(stdos.Args[2])
	case "env":
		env(os.NewHostPath(stdos.Args[2]), stdos.Args[3:]...)
	case "env-prefix":
		envPrefix(os.NewHostPath(stdos.Args[2]), stdos.Args[3])
	case "cat":
		cat(stdos.Args[2])
	case "rm":
		rm(stdos.Args[2])
	case "tempdir":
		tempDir()
	case "stack-abs-path":
		stackAbsPath(stdos.Args[2])
	case "stack-rel-path":
		stackRelPath(stdos.Args[2])
	case "tf-plan-sanitize":
		tfPlanSanitize(stdos.Args[2])
	case "fibonacci":
		fibonacci()
	case "git-normalization":
		gitnorm(stdos.Args[2])
	default:
		log.Fatalf("unknown command %s", stdos.Args[1])
	}
}

// hang will hang the process forever, ignoring any signals.
// It is useful to validate forced kill behavior.
// It will print "ready" when it starts to receive the signals.
// It will print the name of the received signals, which may also be useful in testing.
func hang() {
	signals := make(chan stdos.Signal, 10)
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
	stdos.Exit(code)
}

// env sends os.Environ() on stdout and exits.
func env(rootdir os.Path, names ...string) {
	if len(names) > 0 {
		cwd, err := stdos.Getwd()
		checkerr(err)
		dir := project.PrjAbsPath(rootdir, os.NewHostPath(cwd))

		for _, env := range stdos.Environ() {
			parts := strings.Split(env, "=")
			for _, n := range names {
				if parts[0] == n {
					fmt.Printf("%s: %s\n", dir, strings.Join(parts[1:], "="))
				}
			}
		}
		return
	}
	for _, env := range stdos.Environ() {
		fmt.Println(env)
	}
}

func envPrefix(rootdir os.Path, prefix string) {
	cwd, err := stdos.Getwd()
	checkerr(err)
	dir := project.PrjAbsPath(rootdir, os.NewHostPath(cwd))
	for _, env := range stdos.Environ() {
		parts := strings.Split(env, "=")
		if strings.HasPrefix(parts[0], prefix) {
			fmt.Printf("%s: %s\n", dir, env)
		}
	}
}

// cat the file contents to stdout.
func cat(fname string) {
	bytes, err := stdos.ReadFile(fname)
	checkerr(err)
	fmt.Printf("%s", string(bytes))
}

// rm remove the given path.
func rm(fname string) {
	err := stdos.RemoveAll(fname)
	checkerr(err)
}

// tempdir creates a temporary directory.
func tempDir() {
	tmpdir, err := stdos.MkdirTemp("", "tm-tmpdir")
	checkerr(err)
	fmt.Print(tmpdir)
}

// fibonacci, when called from dir fib.N/, writes the Nth fibonacci number to ./fib.txt.
// It may try to read values from ../fib.N-1/fib.txt and ../fib.N-2/fib.txt, which were previously
// created by running this command in other dirs.
func fibonacci() {
	wd, err := stdos.Getwd()
	checkerr(err)
	dirname := filepath.Base(wd)

	if !strings.HasPrefix(dirname, "fib.") {
		log.Fatalf("fibonacci must be called from dir 'fib.N, was %v'", wd)
	}

	n, err := strconv.ParseInt(dirname[len("fib."):], 10, 64)
	checkerr(err)

	var v int64

	if n == 0 {
		v = 0
	} else if n == 1 {
		v = 1
	} else {
		v = 0
		for _, i := range []int64{n - 1, n - 2} {
			b, err := stdos.ReadFile(fmt.Sprintf("../fib.%v/fib.txt", i))
			checkerr(err)
			ni, err := strconv.ParseInt(string(b), 10, 64)
			checkerr(err)
			v += ni
		}
	}

	checkerr(stdos.WriteFile("fib.txt", []byte(fmt.Sprintf("%v", v)), 0644))
}

func stackAbsPath(base string) {
	cwd, err := stdos.Getwd()
	checkerr(err)
	rel, err := filepath.Rel(base, cwd)
	checkerr(err)
	fmt.Println("/" + filepath.ToSlash(rel))
}

func stackRelPath(base string) {
	cwd, err := stdos.Getwd()
	checkerr(err)
	rel, err := filepath.Rel(base, cwd)
	checkerr(err)
	fmt.Println(filepath.ToSlash(rel))
}

func tfPlanSanitize(fname string) {
	var oldPlan tfjson.Plan
	oldPlanData, err := stdos.ReadFile(fname)
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
		fmt.Fprintf(stdos.Stderr, "error: %v\n", err)
		stdos.Exit(1)
	}
	fmt.Printf("host:  %s\n", repo.Host)
	fmt.Printf("owner: %s\n", repo.Owner)
	fmt.Printf("name:  %s\n", repo.Name)
	fmt.Printf("normalized repository: %s\n", repo.Repo)
}

func checkerr(err error) {
	if err != nil {
		fmt.Fprintf(stdos.Stderr, "%v\n", err)
		stdos.Exit(1)
	}
}

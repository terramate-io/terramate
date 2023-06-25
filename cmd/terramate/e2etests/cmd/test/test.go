// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// test is a test command that implements behaviors that are
// useful when testing terramate run features in a way that reduces
// dependencies on the environment to run the tests.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
)

var pidfile string

func init() {
	flag.StringVar(&pidfile, "pid", "", "pid file to be created at program startup")
}

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("test requires at least one argument: <cmd>")
	}

	if pidfile != "" {
		err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err == nil {
			defer func() {
				err := os.Remove(pidfile)
				if err != nil {
					log.Fatalf("removing pid file: %v", err)
				}
			}()
		} else {
			log.Fatalf("failed to save pid file: %v", err)
		}
	}

	switch os.Args[1] {
	case "hang":
		hang()
	case "env":
		env()
	case "cat":
		cat(os.Args[2])
	case "stack-abs-path":
		stackAbsPath(os.Args[2])
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

// env sends os.Environ() on stdout and exits.
func env() {
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
}

// cat the file contents to stdout.
func cat(fname string) {
	bytes, err := os.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(bytes))
}

func stackAbsPath(base string) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	rel, err := filepath.Rel(base, cwd)
	if err != nil {
		panic(err)
	}
	fmt.Println("/" + filepath.ToSlash(rel))
}

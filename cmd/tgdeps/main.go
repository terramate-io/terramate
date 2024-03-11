// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main implements tgdeps.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tg"
)

func main() {
	trace := flag.Bool("trace", false, "sets log level to trace")
	isJSON := flag.Bool("json", false, "outputs JSON")

	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.Disabled)
	if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
	rootdir, err := os.Getwd()
	abortOnErr(err)

	var dir string
	if len(flag.Args()) == 2 {
		dir = flag.Arg(1)
	} else {
		dir = rootdir
	}

	modules, err := tg.ScanModules(rootdir, project.PrjAbsPath(rootdir, dir), true)
	abortOnErr(err)

	if *isJSON {
		out, err := json.MarshalIndent(modules, "", "  ")
		abortOnErr(err)
		fmt.Printf("%s\n", string(out))
		return
	}

	for _, mod := range modules {
		fmt.Printf("Module: %s\n", mod.Path)
		fmt.Printf("\tSource: %s\n", mod.Source)
		for _, triggerPath := range mod.DependsOn {
			fmt.Printf("\t- %s\n", triggerPath)
		}
	}
}

func abortOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

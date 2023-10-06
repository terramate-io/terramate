// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/terramate-io/terramate/errors"
)

const (
	envNameShowASCII = "TM_TEST_TERRAFORM_SHOW_ASCII_OUTPUT"
	envNameShowJSON  = "TM_TEST_TERRAFORM_SHOW_JSON_OUTPUT"
)

func terraform() {
	switch os.Args[1] {
	case "show":
		fs := flag.NewFlagSet("terraform show", flag.ExitOnError)
		_ = fs.Bool("no-color", false, "-no-color (ignored)")
		isJSON := fs.Bool("json", false, "outputs a json")
		err := fs.Parse(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if fs.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "given args: %v\n", fs.Args())
			fmt.Fprintf(os.Stderr, "usage: %s show <file>\n", os.Args[0])
			os.Exit(1)
		}
		file := fs.Arg(0)
		st, err := os.Lstat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "file not found: %s\n", file)
			os.Exit(1)
		}
		if !st.Mode().IsRegular() {
			fmt.Fprintf(os.Stderr, "not a regular file: %s\n", file)
			os.Exit(1)
		}

		// file exists but ignore it

		var output string
		if !*isJSON {
			output = os.Getenv(envNameShowASCII)
			if output == "" {
				panic(errors.E("please set %s", envNameShowASCII))
			}
		} else {
			output = os.Getenv(envNameShowJSON)
			if output == "" {
				panic(errors.E("please set %s", envNameShowJSON))
			}
		}
		fmt.Print(output)
	default:
		panic(errors.E("unsupported command: %s", os.Args[1]))
	}
}

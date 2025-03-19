// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Terramate is a tool for managing multiple Terraform stacks. Providing stack
// execution orchestration and code generation as a way to share data across
// different stacks.
// For details on how to use it just run:
//
//	terramate --help
package main

import (
	"os"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/ui/tui"
)

func main() {
	cli, err := tui.NewCLI()
	if err != nil {
		panic(errors.E(errors.ErrInternal, "unexpected error"))
	}

	cli.Exec(os.Args[1:])
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Terramate is a tool for managing multiple Terraform stacks. Providing stack
// execution orchestration and code generation as a way to share data across
// different stacks.
// For details on how to use it just run:
//
//	terramate --help
package main

import (
	"os"

	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cmd/terramate/cli"
)

func main() {
	cli.Exec(terramate.Version(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

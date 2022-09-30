// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Terramate is a tool for managing multiple Terraform stacks. Providing stack
// execution orchestration and code generation as a way to share data across
// different stacks.
// For details on how to use it just run:
//	terramate --help
package main

import (
	"os"

	"github.com/mineiros-io/terramate/cmd/terramate/cli"
)

func main() {
	cli.Exec(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

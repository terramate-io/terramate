// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Terramate-ls is a language server.
// For details on how to use it just run:
//
//	terramate-ls --help
package main

import (
	tmls "github.com/terramate-io/terramate/ls"
)

func main() {
	tmls.RunServer()
}

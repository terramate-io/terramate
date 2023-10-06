// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/terramate-io/terramate/errors"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s requires at least one subcommand argument", os.Args[0])
	}

	ownPath, err := os.Executable()
	if err != nil {
		panic(errors.E(err, "cannot detect own path"))
	}

	name := filepath.Base(ownPath)
	if name == "terraform" || name == "terraform.exe" {
		terraform()
	} else {
		helper()
	}
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"log"
	"os"
)

func terragrunt(args ...string) {
	switch args[0] {
	case "show":
		forwardTfStdout := os.Getenv("TERRAGRUNT_FORWARD_TF_STDOUT")
		if forwardTfStdout == "" {
			log.Printf("error: TERRAGRUNT_FORWARD_TF_STDOUT is not set")
			os.Exit(1)
		}
		if forwardTfStdout != "true" {
			log.Printf("error: TERRAGRUNT_FORWARD_TF_STDOUT is not true; got %q", forwardTfStdout)
			os.Exit(1)
		}

		logFormat := os.Getenv("TERRAGRUNT_LOG_FORMAT")
		if logFormat == "" {
			log.Printf("error: TERRAGRUNT_LOG_FORMAT is not set")
			os.Exit(1)
		}
		if logFormat != "bare" {
			log.Printf("error: TERRAGRUNT_LOG_FORMAT is not bare; got %q", logFormat)
			os.Exit(1)
		}

		fmt.Println(`TERRAGRUNT SHOW TEST OUTPUT`)
	}
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/terramate-io/terramate/versions"
)

func terragrunt(args ...string) {
	if len(args) == 0 {
		log.Printf("error: terragrunt requires at least one argument")
		os.Exit(1)
	}

	switch args[0] {
	case "--version":
		version := getTerragruntVersion()
		fmt.Printf("terragrunt version v%s\n", version)
	case "show":
		version := getTerragruntVersion()
		useTG, err := versions.Match(version, ">= 0.73.0", false)
		if err != nil {
			// Default to modern TG_* env when version parsing fails
			useTG = true
		}

		var forwardTfStdout, logFormat string
		var forwardEnvName, logEnvName string

		if useTG {
			forwardEnvName = "TG_FORWARD_TF_STDOUT"
			logEnvName = "TG_LOG_FORMAT"
			forwardTfStdout = os.Getenv(forwardEnvName)
			logFormat = os.Getenv(logEnvName)
		} else {
			forwardEnvName = "TERRAGRUNT_FORWARD_TF_STDOUT"
			logEnvName = "TERRAGRUNT_LOG_FORMAT"
			forwardTfStdout = os.Getenv(forwardEnvName)
			logFormat = os.Getenv(logEnvName)
		}

		if forwardTfStdout == "" {
			log.Printf("error: %s is not set", forwardEnvName)
			os.Exit(1)
		}
		if forwardTfStdout != "true" {
			log.Printf("error: %s is not true; got %q", forwardEnvName, forwardTfStdout)
			os.Exit(1)
		}

		if logFormat == "" {
			log.Printf("error: %s is not set", logEnvName)
			os.Exit(1)
		}
		if logFormat != "bare" {
			log.Printf("error: %s is not bare; got %q", logEnvName, logFormat)
			os.Exit(1)
		}

		fmt.Println(`TERRAGRUNT SHOW TEST OUTPUT`)
	}
}

func getTerragruntVersion() string {
	// Allow tests to override the version
	version := os.Getenv("TM_TEST_HELPER_TERRAGRUNT_VERSION")
	if version == "" {
		// Default to 0.88.0 which matches the installed version in tests
		version = "0.88.0"
	}
	return version
}

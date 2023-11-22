// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build interop

package interop_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
)

func TestMain(m *testing.M) {
	apiHost := os.Getenv("TMC_API_HOST")
	apiURL := os.Getenv("TMC_API_URL")
	if apiHost == "" && apiURL == "" {
		fmt.Printf("The interoperability tests requires exporting either TMC_API_HOST or TMC_API_URL")
		os.Exit(1)
	}
	if apiHost != "" && apiURL != "" {
		fmt.Printf("TMC_API_HOST conflicts with TMC_API_URL")
		os.Exit(1)
	}
	if apiHost == cloud.Host || apiURL == cloud.BaseURL {
		fmt.Printf("Interoperability tests MUST not be executed targeting the product API")
		os.Exit(1)
	}
	packageDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get test working directory: %v", err)
		os.Exit(1)
	}
	// this file is inside cmd/terramate/e2etests/cloud/interop
	// change code below if it's not the case anymore.
	projectRoot := filepath.Join(packageDir, "../../../../..")
	err = runner.Setup(projectRoot)
	if err != nil {
		log.Fatal(err)
	}
	defer runner.Teardown()
	os.Exit(m.Run())
}

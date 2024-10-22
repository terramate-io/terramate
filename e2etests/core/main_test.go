// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
)

func TestMain(m *testing.M) {
	packageDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get test working directory: %v", err)
		os.Exit(1)
	}
	// this file is inside e2etests/core
	// change code below if it's not the case anymore.
	projectRoot := filepath.Join(packageDir, "../..")
	err = Setup(projectRoot)
	if err != nil {
		log.Fatalf("failed to setup e2e tests: %v", err)
	}
	defer Teardown()
	os.Exit(m.Run())
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
)

const testserverJSONFile = "testdata/cloud.data.json"

func TestMain(m *testing.M) {
	packageDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get test working directory: %v", err)
		os.Exit(1)
	}
	// this file is inside cmd/terramate/e2etests/cloud
	// change code below if it's not the case anymore.
	projectRoot := filepath.Join(packageDir, "../../../..")
	err = runner.Setup(projectRoot)
	if err != nil {
		log.Fatal(err)
	}
	defer runner.Teardown()
	os.Exit(m.Run())
}

func nljoin(stacks ...string) string {
	return strings.Join(stacks, "\n") + "\n"
}

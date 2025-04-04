// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/test"
)

const terraformInstallVersion = "1.5.0"

// toolsetTestPath is the path to the directory containing the Terramate binary and
// other tools.
var toolsetTestPath string

// TerraformVersion is the detected or installed Terraform version.
var TerraformVersion string

// TerraformTestPath is the path to the installed terraform binary.
var TerraformTestPath string

var terraformCleanup func()

// HelperPath is the path to the test binary we compiled for test purposes
var HelperPath string

// HelperPathAsHCL is the path to the test binary but as a safe HCL expression
// that's valid in all supported OSs.
var HelperPathAsHCL string

var setupOnce sync.Once

// Setup the e2e test runner.
func Setup(projectRoot string) (err error) {
	setupOnce.Do(func() {
		toolsetTestPath, err = os.MkdirTemp("", "cmd-terramate-test-")
		if err != nil {
			return
		}

		_, err = BuildTerramate(projectRoot, toolsetTestPath)
		if err != nil {
			err = errors.E(err, "failed to setup e2e tests")
			return
		}

		HelperPath, err = BuildTestHelper(projectRoot, toolsetTestPath)
		if err != nil {
			err = errors.E(err, "failed to setup e2e tests")
			return
		}

		// also copies a variant of helper named "terragrunt" for simple e2e testing cases.
		terragruntTestPath := filepath.Join(toolsetTestPath, "terragrunt"+platExeSuffix())
		err = fs.CopyFile(terragruntTestPath, HelperPath)
		if err != nil {
			err = errors.E(err, "failed to make a copy of the helper binary")
			return
		}

		var st os.FileInfo
		st, err = os.Stat(HelperPath)
		if err != nil {
			err = errors.E(err, "failed to stat the helper binary")
			return
		}

		err = test.Chmod(terragruntTestPath, st.Mode())
		if err != nil {
			err = errors.E(err, "failed to set the helper binary executable")
			return
		}

		HelperPathAsHCL = fmt.Sprintf(`${tm_chomp(<<-EOF
		%s
	EOF
	)}`, HelperPath)

		TerraformTestPath, TerraformVersion, terraformCleanup, err = InstallTerraform(terraformInstallVersion)
		if err != nil {
			err = errors.E(err, "failed to setup Terraform binary")
			return
		}
	})

	if err == nil {
		fmt.Fprintf(os.Stderr, "toolsetPath: %s\n", toolsetTestPath)
	}

	return err
}

// Teardown cleanup the runner files.
func Teardown() {
	if err := os.RemoveAll(toolsetTestPath); err != nil {
		fmt.Fprintf(os.Stderr, "cleaning up: failed to remove tmp bin dir %q: %v\n", toolsetTestPath, err)
	}
	if terraformCleanup != nil {
		terraformCleanup()
	}
}

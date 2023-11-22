// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"
	"github.com/terramate-io/terramate/errors"
)

const (
	terraformInstallVersion = "1.5.0"

	testserverJSONFile = "testdata/cloud.data.json"
)

// terramateTestBin is the path to the terramate binary we compiled for test purposes
var terramateTestBin string

// terraformTestBin is the path to the installed terraform binary.
var terraformTestBin string

// terraformVersion is the detected or installed Terraform version.
var terraformVersion string

// testHelperBin is the path to the test binary we compiled for test purposes
var testHelperBin string

// testHelperBinAsHCL is the path to the test binary but as a safe HCL expression
// that's valid in all supported OSs.
var testHelperBinAsHCL string

// The TestMain function creates a terramate binary for testing purposes and
// deletes it after the tests have been run.
func TestMain(m *testing.M) {
	os.Exit(setupAndRunTests(m))
}

func setupAndRunTests(m *testing.M) (status int) {
	binTmpDir, err := os.MkdirTemp("", "cmd-terramate-test-")
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := os.RemoveAll(binTmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "cleaning up: failed to remove tmp bin dir %q: %v\n", binTmpDir, err)
			status = 1
		}
	}()

	goBin, err := lookupGoBin()
	if err != nil {
		log.Printf("failed to setup e2e tests: %v", err)
		return 1
	}

	packageDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get test working directory: %v", err)
		return 1
	}

	// this file is inside cmd/terramate/cli
	// change code below if it's not the case anymore.
	projectRoot := filepath.Join(packageDir, "../../..")
	terramateTestBin, err = buildTerramate(goBin, projectRoot, binTmpDir)
	if err != nil {
		log.Printf("failed to setup e2e tests: %v", err)
		return 1
	}

	testCmdPath := filepath.Join(packageDir, "cmd", "helper")
	testHelperBin, err = buildTestHelper(goBin, testCmdPath, binTmpDir)
	if err != nil {
		log.Printf("failed to setup e2e tests: %v", err)
		return 1
	}

	testHelperBinAsHCL = fmt.Sprintf(`${tm_chomp(<<-EOF
		%s
	EOF
	)}`, testHelperBin)

	tfExecPath, cleanup, err := installTerraform()
	if err != nil {
		log.Printf("failed to setup Terraform binary")
		return 1
	}
	defer cleanup()
	terraformTestBin = tfExecPath

	return m.Run()
}

func buildTestHelper(goBin, testCmdPath, binDir string) (string, error) {
	outBinPath := filepath.Join(binDir, "helper"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
		"-buildvcs=false",
		"-o",
		outBinPath,
		testCmdPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build test helper: %v (output: %s)", err, string(out))
	}
	return outBinPath, err
}

func buildTerramate(goBin, projectRoot, binDir string) (string, error) {
	// We need to build the same way it is built on our Makefile + release process
	// Invoking make here would assume that someone running go test ./... have
	// make installed, so we are keeping the duplication to reduce deps when running tests.
	outBinPath := filepath.Join(binDir, "terramate"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
		"-buildvcs=false",
		"-tags",
		"localhostEndpoints",
		"-race",
		"-o",
		outBinPath,
		filepath.Join(projectRoot, "cmd/terramate"),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build terramate: %v (output: %s)", err, string(out))
	}
	return outBinPath, nil
}

func installTerraform() (string, func(), error) {
	requireVersion := os.Getenv("TM_TEST_TERRAFORM_REQUIRED_VERSION")
	tfExecPath, err := exec.LookPath("terraform")
	if err == nil {
		cmd := exec.Command("terraform", "version")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			terraformVersion = strings.TrimPrefix(strings.TrimSpace(lines[0]), "Terraform v")

			if requireVersion == "" || terraformVersion == requireVersion {
				log.Printf("Terraform detected version: %s", terraformVersion)
				return tfExecPath, func() {}, nil
			}
		}
	}

	installVersion := terraformInstallVersion
	if requireVersion != "" {
		installVersion = requireVersion
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	installer := install.NewInstaller()
	version := version.Must(version.NewVersion(installVersion))

	execPath, err := installer.Install(ctx, []src.Installable{
		&releases.ExactVersion{
			Product: product.Terraform,
			Version: version,
		},
	})
	if err != nil {
		return "", nil, errors.E(err, "installing Terraform")
	}
	terraformVersion = installVersion
	log.Printf("Terraform installed version: %s", terraformVersion)
	return execPath, func() {
		err := installer.Remove(context.Background())
		if err != nil {
			log.Printf("failed to remove terraform installation")
		}
	}, nil
}

func lookupGoBin() (string, error) {
	exeSuffix := platExeSuffix()
	path := filepath.Join(runtime.GOROOT(), "bin", "go"+exeSuffix)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	goBin, err := exec.LookPath("go" + exeSuffix)
	if err != nil {
		return "", fmt.Errorf("cannot find go tool: %v", err.Error())
	}
	return goBin, nil
}

func platExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// terramateTestBin is the path to the terramate binary we compiled for test purposes
var terramateTestBin string

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

	testCmdPath := filepath.Join(packageDir, "cmd", "test")
	testHelperBin, err = buildTestHelper(goBin, testCmdPath, binTmpDir)
	if err != nil {
		log.Printf("failed to setup e2e tests: %v", err)
		return 1
	}

	testHelperBinAsHCL = fmt.Sprintf(`${tm_chomp(<<-EOF
		%s
	EOF
	)}`, testHelperBin)

	return m.Run()
}

func buildTestHelper(goBin, testCmdPath, binDir string) (string, error) {
	outBinPath := filepath.Join(binDir, "test"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
		"-o",
		outBinPath,
		testCmdPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build test helper: %v (output: %s)", err, string(out))
	}
	return outBinPath, nil
}

func buildTerramate(goBin, projectRoot, binDir string) (string, error) {
	// We need to build the same way it is built on our Makefile + release process
	// Invoking make here would assume that someone running go test ./... have
	// make installed, so we are keeping the duplication to reduce deps when running tests.
	outBinPath := filepath.Join(binDir, "terramate"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
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

// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	testCmdPath := filepath.Join(packageDir, "cmd", "_test")
	testHelperBin, err = buildTestHelper(goBin, testCmdPath, binTmpDir)
	if err != nil {
		log.Printf("failed to setup e2e tests: %v", err)
		return 1
	}

	return m.Run()
}

func buildTestHelper(goBin, testCmdPath, binDir string) (string, error) {
	outBinPath := filepath.Join(binDir, "test"+platExeSuffix())
	cmd := exec.Command(
		"gcc",
		filepath.Join(testCmdPath, "test.c"),
		"-o",
		outBinPath,
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
		"--ldflags",
		`-extldflags "-static"`,
		"-o",
		outBinPath,
		filepath.Join(projectRoot, "cmd/terramate"),
	)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

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

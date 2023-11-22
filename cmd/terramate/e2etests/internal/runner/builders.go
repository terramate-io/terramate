// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"github.com/terramate-io/terramate/errors"
)

// BuildTerramate build the Terramate binary from the current filesystem
// tree provided in the projectRoot directory. The binary is built into
// the provided binDir directory.
// The projectRoot is also used to compute the Terramate main package.
func BuildTerramate(projectRoot, binDir string) (string, error) {
	goBin, err := lookupGoBin()
	if err != nil {
		return "", errors.E("failed to setup e2e tests: %v", err)
	}
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
		return "", fmt.Errorf("failed to build terramate (%s): %v (output: %s)", cmd.String(), err, string(out))
	}
	return outBinPath, nil
}

// BuildTestHelper builds the helper tool from the provided projectRoot
// directory into the binDir directory.
// The projectRoot is also used to compute the helper main package.
func BuildTestHelper(projectRoot, binDir string) (string, error) {
	goBin, err := lookupGoBin()
	if err != nil {
		return "", errors.E("failed to setup e2e tests: %v", err)
	}
	outBinPath := filepath.Join(binDir, "helper"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
		"-buildvcs=false",
		"-o",
		outBinPath,
		path.Join(projectRoot, "cmd", "terramate", "e2etests", "cmd", "helper"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build test helper: %v (output: %s)", err, string(out))
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

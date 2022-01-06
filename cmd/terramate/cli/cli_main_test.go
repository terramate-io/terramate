package cli_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var terramateTestBin string

// The TestMain function creates a terramate binary for testing purposes and
// deletes it after the tests have been run.
func TestMain(m *testing.M) {
	os.Exit(setupAndRunTests(m))
}

func setupAndRunTests(m *testing.M) int {
	binTmpdir, err := os.MkdirTemp("", "cmd-terramate-test-")
	if err != nil {
		log.Fatal(err)
	}

	defer os.RemoveAll(binTmpdir)

	goBin, err := lookupGoBin()
	if err != nil {
		log.Fatalf("failed to setup e2e tests: %v", err)
	}

	packageDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get test working directory: %v", err)
	}

	// this file is inside cmd/terramate/cli
	// change code below if it's not the case anymore.
	projectRoot := filepath.Join(packageDir, "../../..")
	terramateTestBin, err = buildTerramate(goBin, projectRoot, binTmpdir)
	if err != nil {
		log.Fatalf("failed to setup e2e tests: %v", err)
	}

	return m.Run()
}

func buildTerramate(goBin string, projectRoot string, binDir string) (string, error) {
	outBinPath := filepath.Join(binDir, "terramate"+platExeSuffix())
	cmd := exec.Command(
		goBin,
		"build",
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

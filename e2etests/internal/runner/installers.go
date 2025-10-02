// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/test"
)

// InstallTerraform installs Terraform (if needed). The preferredVersion is used
// if terraform is not yet installed.
func InstallTerraform(preferredVersion string) (string, string, func(), error) {
	requireVersion := os.Getenv("TM_TEST_TERRAFORM_REQUIRED_VERSION")
	tfExecPath, err := exec.LookPath("terraform")
	if err == nil {
		cmd := exec.Command("terraform", "version")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			terraformVersion := strings.TrimPrefix(strings.TrimSpace(lines[0]), "Terraform v")

			if requireVersion == "" || terraformVersion == requireVersion {
				log.Printf("Terraform detected version: %s", terraformVersion)
				return tfExecPath, terraformVersion, func() {}, nil
			}
		}
	}

	installVersion := preferredVersion
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
		return "", "", nil, errors.E(err, "installing Terraform")
	}
	log.Printf("Terraform installed version: %s", installVersion)
	return execPath, installVersion, func() {
		err := installer.Remove(context.Background())
		if err != nil {
			log.Printf("failed to remove terraform installation")
		}
	}, nil
}

// InstallTerragrunt installs Terragrunt (if needed). The preferredVersion is used
// if terragrunt is not yet installed.
func InstallTerragrunt(preferredVersion string) (string, string, func(), error) {
	requireVersion := os.Getenv("TM_TEST_TERRAGRUNT_REQUIRED_VERSION")
	tgExecPath, err := exec.LookPath("terragrunt")
	if err == nil {
		cmd := exec.Command("terragrunt", "--version")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			lines := strings.Split(string(output), "\n")
			// Terragrunt version output format: "terragrunt version v0.55.21"
			versionLine := strings.TrimSpace(lines[0])
			parts := strings.Fields(versionLine)
			if len(parts) >= 3 {
				terragruntVersion := strings.TrimPrefix(parts[2], "v")
				if requireVersion == "" || terragruntVersion == requireVersion {
					log.Printf("Terragrunt detected version: %s", terragruntVersion)
					return tgExecPath, terragruntVersion, func() {}, nil
				}
			}
		}
	}

	installVersion := preferredVersion
	if requireVersion != "" {
		installVersion = requireVersion
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a temporary directory for the installation
	tmpDir, err := os.MkdirTemp("", "terragrunt-install-")
	if err != nil {
		return "", "", nil, errors.E(err, "creating temp directory for terragrunt installation")
	}

	// Download terragrunt binary
	execPath, err := downloadTerragrunt(ctx, installVersion, tmpDir)
	if err != nil {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			log.Printf("failed to remove temp directory %q: %v", tmpDir, rmErr)
		}
		return "", "", nil, errors.E(err, "installing Terragrunt")
	}

	log.Printf("Terragrunt installed version: %s", installVersion)
	return execPath, installVersion, func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Printf("failed to remove terragrunt installation directory: %v", err)
		}
	}, nil
}

func downloadTerragrunt(ctx context.Context, version, installDir string) (string, error) {
	// Determine the platform-specific binary name and download URL
	var binaryName, downloadURL string

	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			binaryName = "terragrunt"
			downloadURL = fmt.Sprintf("https://github.com/gruntwork-io/terragrunt/releases/download/v%s/terragrunt_darwin_amd64", version)
		case "arm64":
			binaryName = "terragrunt"
			downloadURL = fmt.Sprintf("https://github.com/gruntwork-io/terragrunt/releases/download/v%s/terragrunt_darwin_arm64", version)
		default:
			return "", fmt.Errorf("unsupported architecture for darwin: %s", runtime.GOARCH)
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			binaryName = "terragrunt"
			downloadURL = fmt.Sprintf("https://github.com/gruntwork-io/terragrunt/releases/download/v%s/terragrunt_linux_amd64", version)
		case "arm64":
			binaryName = "terragrunt"
			downloadURL = fmt.Sprintf("https://github.com/gruntwork-io/terragrunt/releases/download/v%s/terragrunt_linux_arm64", version)
		default:
			return "", fmt.Errorf("unsupported architecture for linux: %s", runtime.GOARCH)
		}
	case "windows":
		binaryName = "terragrunt.exe"
		downloadURL = fmt.Sprintf("https://github.com/gruntwork-io/terragrunt/releases/download/v%s/terragrunt_windows_amd64.exe", version)
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating HTTP request: %w", err)
	}

	// Download the binary
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading terragrunt: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close HTTP response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download terragrunt: HTTP %d", resp.StatusCode)
	}

	// Write to file
	execPath := filepath.Join(installDir, binaryName)
	file, err := os.Create(execPath)
	if err != nil {
		return "", fmt.Errorf("creating terragrunt binary file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("failed to close file %q: %v", execPath, cerr)
		}
	}()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("writing terragrunt binary: %w", err)
	}

	// Make executable (use test.Chmod for proper Windows ACL handling)
	err = test.Chmod(execPath, 0755)
	if err != nil {
		return "", fmt.Errorf("making terragrunt binary executable: %w", err)
	}

	return execPath, nil
}

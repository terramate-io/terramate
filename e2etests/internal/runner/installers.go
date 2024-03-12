// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"
	"github.com/terramate-io/terramate/errors"
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

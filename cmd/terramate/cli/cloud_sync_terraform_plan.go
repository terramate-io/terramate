// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/tfjson"
	"github.com/terramate-io/tfjson/sanitize"
)

const terraformShowTimeout = 300 * time.Second

const (
	// ProvisionerTerraform indicates that a plan was created by Terraform.
	ProvisionerTerraform = "terraform"

	// ProvisionerOpenTofu indicates that a plan was created by OpenTofu.
	ProvisionerOpenTofu = "opentofu"
)

func (c *cli) getTerraformChangeset(run stackCloudRun) (*cloud.ChangesetDetails, error) {
	planfile := run.Task.CloudPlanFile
	provisioner := run.Task.CloudPlanProvisioner

	logger := log.With().
		Str("action", "getTerraformChangeset").
		Str("planfile", planfile).
		Stringer("stack", run.Stack.Dir).
		Logger()

	if filepath.IsAbs(planfile) {
		return nil, errors.E(clitest.ErrCloudInvalidTerraformPlanFilePath, "path must be relative to the running stack")
	}

	absPlanFilePath := filepath.Join(run.Stack.HostDir(c.cfg()), planfile)

	// Terragrunt writes the plan to a temporary directory, so we cannot check for its existence.
	if !run.Task.UseTerragrunt {
		_, err := os.Lstat(absPlanFilePath)
		if err != nil {
			return nil, errors.E(err, "checking plan file")
		}
	}

	renderedPlan, err := c.runTerraformShow(run, "-no-color")
	if err != nil {
		logger.Warn().Err(err).Msg("failed to synchronize the ASCII plan output")
	}

	var newJSONPlanData []byte
	jsonPlanData, err := c.runTerraformShow(run, "-no-color", "-json")
	if err == nil {
		newJSONPlanData, err = sanitizeJSONPlan([]byte(jsonPlanData))
		if err != nil {
			logger.Warn().Err(err).Msg("failed to sanitize the JSON plan output")
		}
	} else {
		logger.Warn().Err(err).Msg("failed to synchronize the JSON plan output")
	}

	if renderedPlan == "" && len(newJSONPlanData) == 0 {
		return nil, nil
	}

	var optSerial *int64
	if serial, found := extractTFStateSerial(absPlanFilePath); found {
		optSerial = &serial
	}

	return &cloud.ChangesetDetails{
		Provisioner:    provisioner,
		ChangesetASCII: renderedPlan,
		ChangesetJSON:  string(newJSONPlanData),
		Serial:         optSerial,
	}, nil
}

func sanitizeJSONPlan(jsonPlanBytes []byte) ([]byte, error) {
	var oldPlan tfjson.Plan
	err := json.Unmarshal([]byte(jsonPlanBytes), &oldPlan)
	if err != nil {
		return nil, errors.E(err, "unmarshaling Terraform JSON plan")
	}
	err = oldPlan.Validate()
	if err != nil {
		return nil, errors.E(err, "validating plan file")
	}

	const replaceWith = "__terramate_redacted__"
	newPlan, err := sanitize.SanitizePlanWithValue(&oldPlan, replaceWith)
	if err != nil {
		return nil, errors.E(err)
	}
	newJSONPlanData, err := json.Marshal(newPlan)
	if err != nil {
		return nil, errors.E(err, "failed to marshal sanitized Terraform JSON plan")
	}
	return newJSONPlanData, nil
}

func (c *cli) runTerraformShow(run stackCloudRun, flags ...string) (string, error) {
	var stdout, stderr bytes.Buffer

	planfile := run.Task.CloudPlanFile
	provisioner := run.Task.CloudPlanProvisioner

	var cmdName string
	if run.Task.UseTerragrunt {
		cmdName = "terragrunt"
	} else if provisioner == ProvisionerOpenTofu {
		cmdName = "tofu"
	} else {
		cmdName = "terraform"
	}

	args := []string{"show"}
	args = append(args, flags...)
	if run.Task.UseTerragrunt {
		args = append(args, "--terragrunt-non-interactive")
	}
	args = append(args, planfile)

	ctx, cancel := context.WithTimeout(context.Background(), terraformShowTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = run.Stack.Dir.HostPath(c.rootdir())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = run.Env

	logger := log.With().
		Str("action", "runTerraformShow").
		Str("planfile", planfile).
		Stringer("stack", run.Stack.Dir).
		Str("command", cmd.String()).
		Logger()

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.E(clitest.ErrCloudTerraformPlanFile, "command timed out: %s", cmd.String())
		}

		logger.Error().Str("stderr", stderr.String()).Msg("command stderr")
		return "", errors.E(clitest.ErrCloudTerraformPlanFile, "executing: %s", cmd.String())
	}

	return stdout.String(), nil
}

func extractTFStateSerial(planfile string) (int64, bool) {
	logger := log.With().
		Str("action", "extractTFStateSerial").
		Str("planfile", planfile).
		Logger()

	planReader, err := zip.OpenReader(planfile)
	if err != nil {
		if b, err := os.ReadFile(planfile); err == nil {
			if bytes.HasPrefix(b, []byte("tfplan")) {
				logger.Debug().Msg("plan serial extraction failed: plan file was created with a pre 1.22 version of terraform")
			} else {
				logger.Debug().Err(err).Msg("plan serial extraction failed")
			}
			return 0, false
		}
		return 0, false
	}
	defer planReader.Close() // nolint:errcheck

	var stateFile *zip.File
	for _, file := range planReader.File {
		if file.Name == "tfstate" {
			stateFile = file
			break
		}
	}
	if stateFile == nil {
		logger.Debug().Msg("plan serial extraction failed: no tfstate found")
		return 0, false
	}

	stateReader, err := stateFile.Open()
	if err != nil {
		return 0, false
	}
	defer stateReader.Close() // nolint:errcheck

	type tfstateJSON struct {
		Serial *int64 `json:"serial"`
	}
	var tfstate tfstateJSON

	if err := json.NewDecoder(stateReader).Decode(&tfstate); err != nil {
		logger.Debug().Err(err).Msg("plan serial extraction failed: failed to decode tfstate")
		return 0, false
	}

	if tfstate.Serial == nil {
		logger.Debug().Err(err).Msg("plan serial extraction failed: serial field not found")
		return 0, false
	}

	return *tfstate.Serial, true
}

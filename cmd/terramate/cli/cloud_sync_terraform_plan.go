// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-json/sanitize"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
)

const terraformShowTimeout = 300 * time.Second

func (c *cli) getTerraformChangeset(run stackCloudRun, planfile string) (*cloud.ChangesetDetails, error) {
	logger := log.With().
		Str("action", "getTerraformChangeset").
		Str("planfile", planfile).
		Stringer("stack", run.Stack.Dir).
		Logger()

	if filepath.IsAbs(planfile) {
		return nil, errors.E(clitest.ErrCloudInvalidTerraformPlanFilePath, "path must be relative to the running stack")
	}

	// Terragrunt writes the plan to a temporary directory, so we cannot check for its existence.
	if !run.Task.UseTerragrunt {
		absPlanFilePath := filepath.Join(run.Stack.HostDir(c.cfg()), planfile)
		_, err := os.Lstat(absPlanFilePath)
		if err != nil {
			return nil, errors.E(err, "checking plan file")
		}
	}

	renderedPlan, err := c.runTerraformShow(run, planfile, "-no-color")
	if err != nil {
		logger.Warn().Err(err).Msg("failed to synchronize the ASCII plan output")
	}

	var newJSONPlanData []byte
	jsonPlanData, err := c.runTerraformShow(run, planfile, "-no-color", "-json")
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

	return &cloud.ChangesetDetails{
		Provisioner:    "terraform",
		ChangesetASCII: renderedPlan,
		ChangesetJSON:  string(newJSONPlanData),
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

func (c *cli) runTerraformShow(run stackCloudRun, planfile string, flags ...string) (string, error) {
	var stdout, stderr bytes.Buffer

	cmdName := "terraform"
	if run.Task.UseTerragrunt {
		cmdName = "terragrunt"
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

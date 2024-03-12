// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-json/sanitize"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
)

const terraformShowTimeout = 60 * time.Second

func (c *cli) cloudSyncDriftStatus(run runContext, res runResult, err error) {
	st := run.Stack

	logger := log.With().
		Str("action", "cloudSyncDriftStatus").
		Stringer("stack", st.Dir).
		Int("exit_code", res.ExitCode).
		Strs("command", run.Cmd).
		Err(err).
		Logger()

	var status drift.Status
	switch {
	case res.ExitCode == 0:
		status = drift.OK
	case res.ExitCode == 2:
		status = drift.Drifted
	case res.ExitCode == 1 || res.ExitCode > 2 || errors.IsAnyKind(err, ErrRunCommandNotFound, ErrRunFailed):
		status = drift.Failed
	default:
		// ignore exit codes < 0
		logger.Debug().Msg("skipping drift sync")
		return
	}

	var driftDetails *cloud.ChangesetDetails

	if planfile := c.parsedArgs.Run.CloudSyncTerraformPlanFile; planfile != "" {
		var err error
		driftDetails, err = c.getTerraformChangeset(run, planfile)
		if err != nil {
			logger.Error().Err(err).Msg(clitest.CloudSkippingTerraformPlanSync)
		}
	}

	logger = logger.With().
		Stringer("drift_status", status).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	_, err = c.cloud.client.CreateStackDrift(ctx, c.cloud.run.orgUUID, cloud.DriftStackPayloadRequest{
		Stack: cloud.Stack{
			Repository:      c.prj.prettyRepo(),
			DefaultBranch:   c.prj.gitcfg().DefaultBranch,
			Path:            st.Dir.String(),
			MetaID:          strings.ToLower(st.ID),
			MetaName:        st.Name,
			MetaDescription: st.Description,
			MetaTags:        st.Tags,
		},
		Status:     status,
		Details:    driftDetails,
		Metadata:   c.cloud.run.metadata,
		StartedAt:  res.StartedAt,
		FinishedAt: res.FinishedAt,
		Command:    run.Cmd,
	})

	if err != nil {
		logger.Error().Err(err).Msg(clitest.CloudSyncDriftFailedMessage)
	} else {
		logger.Debug().Msg("synced drift_status successfully")
	}
}

func (c *cli) getTerraformChangeset(run runContext, planfile string) (*cloud.ChangesetDetails, error) {
	logger := log.With().
		Str("action", "getTerraformChangeset").
		Str("planfile", planfile).
		Stringer("stack", run.Stack.Dir).
		Logger()

	if filepath.IsAbs(planfile) {
		return nil, errors.E(clitest.ErrCloudInvalidTerraformPlanFilePath, "path must be relative to the running stack")
	}

	absPlanFilePath := filepath.Join(run.Stack.HostDir(c.cfg()), planfile)
	_, err := os.Lstat(absPlanFilePath)
	if err != nil {
		return nil, errors.E(err, "checking plan file")
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

func (c *cli) runTerraformShow(run runContext, planfile string, flags ...string) (string, error) {
	var stdout, stderr bytes.Buffer

	args := []string{
		"show",
	}
	args = append(args, flags...)
	args = append(args, planfile)

	ctx, cancel := context.WithTimeout(context.Background(), terraformShowTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = run.Stack.Dir.HostPath(c.rootdir())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger := log.With().
		Str("action", "runTerraformShow").
		Str("planfile", planfile).
		Stringer("stack", run.Stack.Dir).
		Str("command", cmd.String()).
		Logger()

	err := cmd.Run()
	if err != nil {
		logger.Error().Str("stderr", stderr.String()).Msg("command stderr")
		return "", errors.E(clitest.ErrCloudTerraformPlanFile, "executing: %s", cmd.String())
	}

	return stdout.String(), nil
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
)

func (c *cli) cloudSyncDriftStatus(runContext ExecContext, exitCode int, err error) {
	st := runContext.Stack

	logger := log.With().
		Str("action", "cloudSyncDriftStatus").
		Stringer("stack", st.Dir).
		Int("exit_code", exitCode).
		Strs("command", runContext.Cmd).
		Err(err).
		Logger()

	var status stack.Status
	switch {
	case exitCode == 0:
		status = stack.OK
	case exitCode == 2:
		status = stack.Drifted
	case exitCode == 1 || exitCode > 2 || errors.IsAnyKind(err, ErrRunCommandNotFound, ErrRunFailed):
		status = stack.Failed
	default:
		// ignore exit codes < 0
		logger.Debug().Msg("skipping drift sync")
		return
	}

	var driftDetails *cloud.DriftDetails

	if planfile := c.parsedArgs.Run.CloudSyncTerraformPlanFile; planfile != "" {
		var err error
		driftDetails, err = c.getTerraformDriftDetails(runContext, planfile)
		if err != nil {
			logger.Error().Err(err).Msg("skipping the sync of Terraform plan details")
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
			Path:            st.Dir.String(),
			MetaID:          st.ID,
			MetaName:        st.Name,
			MetaDescription: st.Description,
			MetaTags:        st.Tags,
		},
		Status:   status,
		Details:  driftDetails,
		Metadata: c.cloud.run.metadata,
		Command:  runContext.Cmd,
	})

	if err != nil {
		logger.Error().Err(err).Msg(clitest.CloudSyncDriftFailedMessage)
	} else {
		logger.Debug().Msg("synced drift_status successfully")
	}
}

func (c *cli) getTerraformDriftDetails(runContext ExecContext, planfile string) (*cloud.DriftDetails, error) {
	logger := log.With().
		Str("action", "getTerraformDriftDetails").
		Str("planfile", planfile).
		Stringer("stack", runContext.Stack.Dir).
		Logger()

	if filepath.IsAbs(planfile) {
		return nil, errors.E(clitest.ErrCloudInvalidTerraformPlanFilePath, "path must be relative to the running stack")
	}

	var stdout, stderr bytes.Buffer

	const tfShowTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), tfShowTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terraform", "show", "-no-color", planfile)
	cmd.Dir = runContext.Stack.Dir.HostPath(c.rootdir())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Trace().Msgf("executing %s", cmd.String())
	err := cmd.Run()
	if err != nil {
		logger.Debug().Str("stderr", stderr.String()).Msg("command stderr")

		return nil, errors.E(clitest.ErrCloudTerraformPlanFile, "executing: %s", cmd.String())
	}

	logger.Trace().Msg("drift details gathered successfully")

	return &cloud.DriftDetails{
		Provisioner:    "terraform",
		ChangesetASCII: stdout.String(),
	}, nil
}

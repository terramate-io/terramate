// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/errors"
)

func (c *cli) cloudSyncDriftStatus(run stackCloudRun, res runResult, err error) {
	st := run.Stack

	logger := log.With().
		Str("action", "cloudSyncDriftStatus").
		Stringer("stack", st.Dir).
		Int("exit_code", res.ExitCode).
		Strs("command", run.Task.Cmd).
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

	if planfile := run.Task.CloudSyncTerraformPlanFile; planfile != "" {
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
		Command:    run.Task.Cmd,
	})

	if err != nil {
		logger.Error().Err(err).Msg(clitest.CloudSyncDriftFailedMessage)
	} else {
		logger.Debug().Msg("synced drift_status successfully")
	}
}

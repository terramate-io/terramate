// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

func cloudSyncDriftStatus(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, res engine.RunResult, err error) {
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
	case res.ExitCode == 1 || res.ExitCode > 2 || errors.IsAnyKind(err, engine.ErrRunCommandNotExecuted, engine.ErrRunFailed):
		status = drift.Failed
	default:
		// ignore exit codes < 0
		logger.Debug().Msg("skipping drift sync")
		return
	}

	var driftDetails *resources.ChangesetDetails

	if run.Task.CloudPlanFile != "" {
		var err error
		driftDetails, err = getTerraformChangeset(e, run)
		if err != nil {
			logger.Error().Err(err).Msg(clitest.CloudSkippingTerraformPlanSync)
		}
	}

	logger = logger.With().
		Stringer("drift_status", status).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()

	prettyRepo, err := e.Project().PrettyRepo()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get pretty repo")
		return
	}

	_, err = e.CloudClient().CreateStackDrift(ctx, e.CloudState().Org.UUID, resources.DriftStackPayloadRequest{
		Stack: resources.Stack{
			Repository:      prettyRepo,
			Target:          run.Task.CloudTarget,
			FromTarget:      run.Task.CloudFromTarget,
			DefaultBranch:   e.Project().GitConfig().DefaultBranch,
			Path:            st.Dir.String(),
			MetaID:          strings.ToLower(st.ID),
			MetaName:        st.Name,
			MetaDescription: st.Description,
			MetaTags:        st.Tags,
		},
		Status:     status,
		Details:    driftDetails,
		Metadata:   state.Metadata,
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

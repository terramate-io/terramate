// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

func doDriftBefore(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState) {
	st := run.Stack

	logger := log.With().
		Str("action", "doDriftBefore").
		Stringer("stack", st.Dir).
		Strs("command", run.Task.Cmd).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()

	prettyRepo, err := e.Project().PrettyRepo()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get pretty repo")
		e.DisableCloudFeatures(errors.E(errors.ErrInternal, "failed to get pretty repo"))
		return
	}

	startedAt := time.Now().UTC()
	// Create drift with initial status (Drifted is a placeholder, will be updated later)
	// The actual status will be set in cloudSyncDriftStatus
	response, err := e.CloudClient().CreateStackDrift(ctx, e.CloudState().Org.UUID, resources.DriftCheckRunStartPayloadRequest{
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
		Metadata:  state.Metadata,
		StartedAt: &startedAt,
		Command:   run.Task.Cmd,
	})

	if err != nil {
		logger.Error().Err(err).Msg("failed to create drift")
		e.DisableCloudFeatures(errors.E(errors.ErrInternal, clitest.CloudSyncDriftFailedMessage))
		return
	}

	state.SetMeta2DriftUUID(st.ID, response.DriftUUID)
	logger.Debug().
		Str("drift_uuid", string(response.DriftUUID)).
		Msg("Created drift")
}

func doDriftAfter(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, res engine.RunResult, err error) {
	st := run.Stack

	logger := log.With().
		Str("action", "doDriftAfter").
		Stringer("stack", st.Dir).
		Str("stack_id", st.ID).
		Int("exit_code", res.ExitCode).
		Strs("command", run.Task.Cmd).
		Err(err).
		Logger()

	driftUUID, found := state.CloudDriftUUID(st.ID)
	if !found || driftUUID == "" || driftUUID == resources.UUID(uuid.Nil.String()) {
		// That could mean the doDriftBefore was never run as the command never ran.
		// In this case we create the drift and we create the drift run and immediately failed.
		if errors.IsAnyKind(err, engine.ErrRunCommandNotExecuted, engine.ErrRunFailed) {
			doDriftBefore(e, run, state)
			driftUUID, found = state.CloudDriftUUID(st.ID)
		}

		if !found || driftUUID == "" || driftUUID == resources.UUID(uuid.Nil.String()) {
			logger.Error().Msg("missing drift UUID for stack ID")
			return
		}
	}

	logger = logger.With().Str("drift_uuid", string(driftUUID)).Logger()

	var status drift.Status
	switch {
	case res.ExitCode == 0:
		status = drift.OK
	case res.ExitCode == 2:
		status = drift.Drifted
	case res.ExitCode == 1 || res.ExitCode > 2 || errors.IsAnyKind(err, engine.ErrRunCommandNotExecuted, engine.ErrRunFailed, engine.ErrRunCanceled):
		status = drift.Failed
	default:
		// ignore exit codes < 0
		logger.Debug().Msg("skipping drift sync")
		return
	}

	var changeset *resources.ChangesetDetails

	if run.Task.CloudPlanFile != "" {
		var err error
		changeset, err = getTerraformChangeset(e, run)
		if err != nil {
			logger.Error().Err(err).Msg(clitest.CloudSkippingTerraformPlanSync)
		}
	}

	logger = logger.With().
		Stringer("drift_status", status).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()

	finishedAt := time.Now().UTC()
	if res.FinishedAt != nil {
		finishedAt = *res.FinishedAt
	}

	err = e.CloudClient().UpdateStackDrift(ctx, e.CloudState().Org.UUID, driftUUID, resources.UpdateDriftPayloadRequest{
		Status:    status,
		UpdatedAt: finishedAt,
		Changeset: changeset,
	})

	if err != nil {
		logger.Error().Err(err).Msg(clitest.CloudSyncDriftFailedMessage)
	} else {
		logger.Debug().Msg("synced drift_status successfully")
	}
}

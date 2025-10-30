// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/engine"
)

// BeforeRun is called before a cloud run.
func BeforeRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		doCloudSyncDeployment(e, run, state, deployment.Running)
	}

	if run.Task.CloudSyncDriftStatus {
		doDriftBefore(e, run, state)
	}

	if run.Task.CloudSyncPreview {
		doPreviewBefore(e, run, state)
	}
}

// AfterRun is called after a cloud run.
func AfterRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, res engine.RunResult, err error) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		cloudSyncDeployment(e, run, state, err)
	}

	if run.Task.CloudSyncDriftStatus {
		doDriftAfter(e, run, state, res, err)
	}

	if run.Task.CloudSyncPreview {
		doPreviewAfter(e, run, state, res, err)
	}
}

// Logs synchronizes the logs of a command with the Terramate Cloud.
func Logs(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, state *CloudRunState, logs resources.CommandLogs) {
	if !e.IsCloudEnabled() {
		return
	}
	data, _ := json.Marshal(logs)
	logger.Debug().RawJSON("logs", data).Msg("synchronizing logs")
	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	stackID, _ := state.StackCloudID(run.Stack.ID)
	stackPreviewID, _ := state.CloudPreviewID(run.Stack.ID)
	err := e.CloudClient().SyncCommandLogs(
		ctx, e.CloudState().Org.UUID, stackID, state.RunUUID, logs, stackPreviewID,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to sync logs")
	}
}

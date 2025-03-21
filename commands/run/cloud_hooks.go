// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/engine"
)

// CloudSyncBefore is called before a cloud run.
func CloudSyncBefore(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		doCloudSyncDeployment(e, run, state, deployment.Running)
	}

	if run.Task.CloudSyncPreview {
		doPreviewBefore(e, run, state)
	}
}

// CloudSyncAfter is called after a cloud run.
func CloudSyncAfter(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, res engine.RunResult, err error) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		cloudSyncDeployment(e, run, state, err)
	}

	if run.Task.CloudSyncDriftStatus {
		cloudSyncDriftStatus(e, run, state, res, err)
	}

	if run.Task.CloudSyncPreview {
		doPreviewAfter(e, run, state, res, err)
	}
}

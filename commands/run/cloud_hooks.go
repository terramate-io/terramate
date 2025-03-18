package run

import (
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/engine"
)

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

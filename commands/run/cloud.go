// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"github.com/hashicorp/go-uuid"
	"github.com/terramate-io/terramate/cloud"
)

const (
	cloudFeatStatus          = "--status' is a Terramate Cloud feature to filter stacks that failed to deploy or have drifted."
	cloudFeatSyncDeployment  = "'--sync-deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatSyncDriftStatus = "'--sync-drift-status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatSyncPreview     = "'--sync-preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

func (s *Spec) checkCloudSync() error {
	if !s.SyncDeployment && !s.SyncDriftStatus && !s.SyncPreview {
		return nil
	}

	var feats []string
	if s.SyncDeployment {
		feats = append(feats, cloudFeatSyncDeployment)
	}
	if s.SyncDriftStatus {
		feats = append(feats, cloudFeatSyncDriftStatus)
	}
	if s.SyncPreview {
		feats = append(feats, cloudFeatSyncPreview)
	}

	err := s.Engine.SetupCloudConfig(feats)
	s.Engine.HandleCloudCriticalError(err)

	if s.Engine.IsCloudDisabled() {
		return
	}

	if c.parsedArgs.Run.SyncDeployment {
		uuid, err := uuid.GenerateUUID()
		c.handleCriticalError(err)
		c.cloud.run.runUUID = cloud.UUID(uuid)
	}
}

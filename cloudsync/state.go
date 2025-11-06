// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"strings"
	"sync"

	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

const (
	// ProvisionerTerraform indicates that a plan was created by Terraform.
	ProvisionerTerraform = "terraform"

	// ProvisionerOpenTofu indicates that a plan was created by OpenTofu.
	ProvisionerOpenTofu = "opentofu"
)

// CloudRunState represents the state of the current run.
type CloudRunState struct {
	sync.RWMutex

	RunUUID resources.UUID

	stackMeta2ID map[string]int64
	// stackMeta2PreviewIDs is a map of stack.ID to stackPreview.ID
	stackMeta2PreviewIDs map[string]string
	// stackMeta2DriftUUIDs is a map of stack.ID to drift.UUID
	stackMeta2DriftUUIDs map[string]resources.UUID
	ReviewRequest        *resources.ReviewRequest
	RREvent              struct {
		PushedAt  *int64
		CommitSHA string
	}
	Metadata *resources.DeploymentMetadata
}

// SetMeta2CloudID sets the cloud ID of a stack given its metadata ID.
func (rs *CloudRunState) SetMeta2CloudID(metaID string, id int64) {
	rs.Lock()
	defer rs.Unlock()
	if rs.stackMeta2ID == nil {
		rs.stackMeta2ID = make(map[string]int64)
	}
	rs.stackMeta2ID[strings.ToLower(metaID)] = id
}

// StackCloudID returns the cloud ID of a stack given its metadata ID.
func (rs *CloudRunState) StackCloudID(metaID string) (int64, bool) {
	rs.RLock()
	defer rs.RUnlock()
	id, ok := rs.stackMeta2ID[strings.ToLower(metaID)]
	return id, ok
}

// SetMeta2PreviewID sets the cloud preview ID of a stack given its metadata ID.
func (rs *CloudRunState) SetMeta2PreviewID(metaID string, previewID string) {
	rs.Lock()
	defer rs.Unlock()
	if rs.stackMeta2PreviewIDs == nil {
		rs.stackMeta2PreviewIDs = make(map[string]string)
	}
	rs.stackMeta2PreviewIDs[strings.ToLower(metaID)] = previewID
}

// CloudPreviewID returns the cloud preview ID of a stack given its metadata ID.
func (rs *CloudRunState) CloudPreviewID(metaID string) (string, bool) {
	rs.RLock()
	defer rs.RUnlock()
	id, ok := rs.stackMeta2PreviewIDs[strings.ToLower(metaID)]
	return id, ok
}

// SetMeta2DriftUUID sets the cloud drift UUID of a stack given its metadata ID.
func (rs *CloudRunState) SetMeta2DriftUUID(metaID string, driftUUID resources.UUID) {
	rs.Lock()
	defer rs.Unlock()
	if rs.stackMeta2DriftUUIDs == nil {
		rs.stackMeta2DriftUUIDs = make(map[string]resources.UUID)
	}
	rs.stackMeta2DriftUUIDs[strings.ToLower(metaID)] = driftUUID
}

// CloudDriftUUID returns the cloud drift UUID of a stack given its metadata ID.
func (rs *CloudRunState) CloudDriftUUID(metaID string) (resources.UUID, bool) {
	rs.RLock()
	defer rs.RUnlock()
	id, ok := rs.stackMeta2DriftUUIDs[strings.ToLower(metaID)]
	return id, ok
}

func cloudError() error {
	return errors.E(clitest.ErrCloud)
}

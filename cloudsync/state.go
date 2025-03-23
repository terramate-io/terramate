// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"strings"

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
	RunUUID resources.UUID

	StackMeta2ID map[string]int64
	// stackPreviews is a map of stack.ID to stackPreview.ID
	StackMeta2PreviewIDs map[string]string
	ReviewRequest        *resources.ReviewRequest
	RREvent              struct {
		PushedAt  *int64
		CommitSHA string
	}
	Metadata *resources.DeploymentMetadata
}

// SetMeta2CloudID sets the cloud ID of a stack given its metadata ID.
func (rs *CloudRunState) SetMeta2CloudID(metaID string, id int64) {
	if rs.StackMeta2ID == nil {
		rs.StackMeta2ID = make(map[string]int64)
	}
	rs.StackMeta2ID[strings.ToLower(metaID)] = id
}

// StackCloudID returns the cloud ID of a stack given its metadata ID.
func (rs CloudRunState) StackCloudID(metaID string) (int64, bool) {
	id, ok := rs.StackMeta2ID[strings.ToLower(metaID)]
	return id, ok
}

// SetMeta2PreviewID sets the cloud preview ID of a stack given its metadata ID.
func (rs *CloudRunState) SetMeta2PreviewID(metaID string, previewID string) {
	if rs.StackMeta2PreviewIDs == nil {
		rs.StackMeta2PreviewIDs = make(map[string]string)
	}
	rs.StackMeta2PreviewIDs[strings.ToLower(metaID)] = previewID
}

// CloudPreviewID returns the cloud preview ID of a stack given its metadata ID.
func (rs CloudRunState) CloudPreviewID(metaID string) (string, bool) {
	id, ok := rs.StackMeta2PreviewIDs[strings.ToLower(metaID)]
	return id, ok
}

func cloudError() error {
	return errors.E(clitest.ErrCloud)
}

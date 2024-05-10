// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/errors"
)

type (
	// PreviewStacks is a list of stack objects for the request payload
	PreviewStacks []PreviewStack

	// PreviewStack represents the stack object of the request payload
	// type for the creation of stack deployments.
	PreviewStack struct {
		Stack

		PreviewStatus preview.StackStatus `json:"preview_status"`
		Cmd           []string            `json:"cmd,omitempty"`
	}
	// CreatePreviewPayloadRequest is the request payload for the creation of
	// stack deployments.
	CreatePreviewPayloadRequest struct {
		CommitSHA       string              `json:"commit_sha"`
		PushedAt        int64               `json:"pushed_at"`
		UpdatedAt       int64               `json:"updated_at"`
		Technology      string              `json:"technology"`
		TechnologyLayer string              `json:"technology_layer"`
		ReviewRequest   *ReviewRequest      `json:"review_request,omitempty"`
		Metadata        *DeploymentMetadata `json:"metadata,omitempty"`
		Stacks          PreviewStacks       `json:"stacks"`
	}

	// ResponsePreviewStacks is a list of stack objects in the response payload
	ResponsePreviewStacks []ResponsePreviewStack

	// ResponsePreviewStack represents a specific stack in the preview response.
	ResponsePreviewStack struct {
		MetaID         string `json:"meta_id"`
		StackPreviewID string `json:"stack_preview_id"`
	}

	// CreatePreviewResponse represents the deployment creation response item.
	CreatePreviewResponse struct {
		PreviewID string                `json:"preview_id"`
		Stacks    ResponsePreviewStacks `json:"stacks"`
	}

	// UpdateStackPreviewPayloadRequest is the request payload for the update of
	// stack previews.
	UpdateStackPreviewPayloadRequest struct {
		Status           string            `json:"status"`
		ChangesetDetails *ChangesetDetails `json:"changeset_details,omitempty"`
	}
)

// Validate the UpdateStackPreviewPayloadRequest object.
func (r UpdateStackPreviewPayloadRequest) Validate() error {
	errs := errors.L()
	if r.Status == "" {
		errs.Append(errors.E("status is required"))
	}

	if r.ChangesetDetails != nil {
		if err := r.ChangesetDetails.Validate(); err != nil {
			errs.Append(err)
		}
	}
	return errs.AsError()
}

// Validate the ResponsePreviewStacks object.
func (s ResponsePreviewStacks) Validate() error {
	errs := errors.L()

	for i, stack := range s {
		if stack.MetaID == "" {
			errs.Append(errors.E(`missing "meta_id" field for stack[%d]`, i))
		}
		if stack.StackPreviewID == "" {
			errs.Append(errors.E(`missing "stack_preview_id" field for stack[%d]`, i))
		}
	}

	return errs.AsError()
}

// Validate the PreviewStacks object.
func (s PreviewStacks) Validate() error {
	errs := errors.L()
	for i, stack := range s {
		if stack.PreviewStatus == "" {
			errs.Append(errors.E(`missing "preview_status" field for stack[%d]`, i))
		}
		if stack.Cmd == nil {
			errs.Append(errors.E(`missing "cmd" field for stack[%d]`, i))
		}
		if err := stack.Stack.Validate(); err != nil {
			errs.Append(errors.E(err, "invalid attributes for stack[%d]", i))
		}
	}

	return errs.AsError()
}

// Validate the CreatePreviewPayloadRequest object.
func (r CreatePreviewPayloadRequest) Validate() error {
	errs := errors.L()
	if r.Technology == "" {
		errs.Append(errors.E(`missing "technology" field`))
	}
	if r.TechnologyLayer == "" {
		errs.Append(errors.E(`missing "technology_layer" field`))
	}
	if r.UpdatedAt == 0 {
		errs.Append(errors.E(`missing "updated_at" field`))
	}
	if r.PushedAt == 0 {
		errs.Append(errors.E(`missing "pushed_at" field`))
	}
	if r.CommitSHA == "" {
		errs.Append(errors.E(`missing "commit_sha" field`))
	}
	if r.Stacks == nil {
		errs.Append(errors.E(`missing "stacks" field`))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}
	if r.ReviewRequest == nil {
		errs.Append(errors.E(`missing "review_request" field`))
	} else {
		if err := r.ReviewRequest.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

// Validate validates the CreatePreviewResponse payload
func (r CreatePreviewResponse) Validate() error {
	errs := errors.L()

	if r.PreviewID == "" {
		errs.Append(errors.E(`missing "preview_id" field`))
	}

	if r.Stacks == nil {
		errs.Append(errors.E(`missing "stacks" field`))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

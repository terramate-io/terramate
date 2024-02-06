// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import "github.com/terramate-io/terramate/errors"

type (
	// PreviewStacks is a list of stack objects for the request payload
	PreviewStacks []PreviewStack

	// PreviewStack represents the stack object of the request payload
	// type for the creation of stack deployments.
	PreviewStack struct {
		PreviewStatus   string   `json:"preview_status"`
		Repository      string   `json:"repository"`
		Path            string   `json:"path"`
		MetaID          string   `json:"meta_id"`
		MetaName        string   `json:"meta_name"`
		MetaDescription string   `json:"meta_description"`
		MetaTags        []string `json:"meta_tags"`
		DefaultBranch   string   `json:"default_branch"`
		Cmd             []string `json:"cmd"`
	}
	// CreatePreviewPayloadRequest is the request payload for the creation of
	// stack deployments.
	CreatePreviewPayloadRequest struct {
		UpdatedAt       int64                    `json:"updated_at"`
		Technology      string                   `json:"technology"`
		TechnologyLayer string                   `json:"technology_layer"`
		ReviewRequest   *DeploymentReviewRequest `json:"review_request,omitempty"`
		Metadata        *DeploymentMetadata      `json:"metadata,omitempty"`
		Stacks          PreviewStacks            `json:"stacks"`
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
)

// Validate the ResponsePreviewStacks object.
func (s ResponsePreviewStacks) Validate() error {
	errs := errors.L()

	for i, stack := range s {
		if stack.MetaID == "" {
			errs.Append(errors.E("meta_id is required for stack[%d]", i))
		}
		if stack.StackPreviewID == "" {
			errs.Append(errors.E("stack_preview_id is required for stack[%d]", i))
		}
	}

	return errs.AsError()
}

// Validate the PreviewStacks object.
func (s PreviewStacks) Validate() error {
	errs := errors.L()
	for i, stack := range s {
		if stack.PreviewStatus == "" {
			errs.Append(errors.E("preview_status is required for stack[%d]", i))
		}
		if stack.Repository == "" {
			errs.Append(errors.E("repository is required for stack[%d]", i))
		}
		if stack.Path == "" {
			errs.Append(errors.E("path is required for stack[%d]", i))
		}
		if stack.DefaultBranch == "" {
			errs.Append(errors.E("default_branch is required for stack[%d]", i))
		}
		if stack.MetaID == "" {
			errs.Append(errors.E("meta_id is required required for stack[%d]", i))
		}
		if stack.Cmd == nil {
			errs.Append(errors.E("cmd is required for stack[%d]", i))
		}
	}

	return errs.AsError()
}

// Validate the CreatePreviewPayloadRequest object.
func (r CreatePreviewPayloadRequest) Validate() error {
	errs := errors.L()
	if r.Technology == "" {
		errs.Append(errors.E("technology is required"))
	}
	if r.TechnologyLayer == "" {
		errs.Append(errors.E("technology_layer is required"))
	}
	if r.UpdatedAt == 0 {
		errs.Append(errors.E("updated_at is required"))
	}
	if r.Stacks == nil {
		errs.Append(errors.E("stacks is required"))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}
	if r.ReviewRequest == nil {
		errs.Append(errors.E("review_request is required"))
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
		errs.Append(errors.E("preview_id is required"))
	}

	if r.Stacks == nil {
		errs.Append(errors.E("stacks is required"))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	"strings"
	"time"

	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

// RunContext is the context for a run
type RunContext struct {
	Stack *config.Stack
	Cmd   []string
}

// CreatePreviewOpts is the options for the CreatePreview function
type CreatePreviewOpts struct {
	Runs            []RunContext
	AffectedStacks  map[string]*config.Stack
	OrgUUID         UUID
	UpdatedAt       int64
	Technology      string
	TechnologyLayer string
	Repository      string
	DefaultBranch   string
	ReviewRequest   *ReviewRequest
	Metadata        *DeploymentMetadata
}

// CreatedPreview is the result of the CreatePreview function
type CreatedPreview struct {
	ID                    string
	StackPreviewsByMetaID map[string]string
}

// CreateStackPreviewOpts is the options for the CreateStackPreview function
type CreateStackPreviewOpts struct {
	OrgUUID          UUID
	StackPreviewID   string
	ChangesetDetails *ChangesetDetails
}

// CreatePreview creates a new preview in the cloud
func (c *Client) CreatePreview(timeout time.Duration, opts CreatePreviewOpts) (*CreatedPreview, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	payload := CreatePreviewPayloadRequest{
		UpdatedAt:       opts.UpdatedAt,
		Technology:      opts.Technology,
		TechnologyLayer: opts.TechnologyLayer,
		ReviewRequest:   opts.ReviewRequest,
		Metadata:        opts.Metadata,
		Stacks:          []PreviewStack{},
	}

	previewStacksMap := map[string]RunContext{}
	for _, run := range opts.Runs {
		previewStacksMap[run.Stack.ID] = run
	}

	// loop over all affected stacks, if an item is present in the
	// previewStacksMap, use the preview status and cmd from there
	for _, affectedStack := range opts.AffectedStacks {
		stack := PreviewStack{
			PreviewStatus: preview.StatusAffected,
			Cmd:           []string{},
			Stack: Stack{
				Repository:      opts.Repository,
				Path:            affectedStack.Dir.String(),
				MetaID:          strings.ToLower(affectedStack.ID),
				MetaName:        affectedStack.Name,
				MetaDescription: affectedStack.Description,
				MetaTags:        affectedStack.Tags,
				DefaultBranch:   opts.DefaultBranch,
			},
		}

		if previewStack, found := previewStacksMap[affectedStack.ID]; found {
			stack.PreviewStatus = preview.StatusPending
			stack.Cmd = previewStack.Cmd
		}
		payload.Stacks = append(payload.Stacks, stack)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := c.createPreview(ctx, opts.OrgUUID, payload)
	if err != nil {
		return nil, err
	}

	if len(res.Stacks) != len(opts.AffectedStacks) {
		return nil, errors.E("the backend respond with an invalid number of stacks in the deployment, got %d, expected %d",
			len(res.Stacks), len(opts.AffectedStacks),
			err,
		)
	}

	stacks := map[string]string{}
	for _, r := range res.Stacks {
		if r.MetaID == "" {
			return nil, errors.E("backend returned empty meta_id")
		}
		stacks[r.MetaID] = r.StackPreviewID
	}

	return &CreatedPreview{
		ID:                    res.PreviewID,
		StackPreviewsByMetaID: stacks,
	}, nil
}

func (o CreatePreviewOpts) validate() error {
	errs := errors.L()

	if len(o.AffectedStacks) == 0 || len(o.Runs) == 0 {
		errs.Append(errors.E("no affected stacks or runs provided"))
	}

	if string(o.OrgUUID) == "" {
		errs.Append(errors.E("org uuid is empty"))
	}

	return errs.AsError()
}

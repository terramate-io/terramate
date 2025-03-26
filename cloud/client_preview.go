// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	"path"

	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/http"
)

const (
	// PreviewsPath is the previews endpoints base path.
	PreviewsPath = "/v1/previews"
	// StackPreviewsPath is the stack previews endpoint base path.
	StackPreviewsPath = "/v1/stack_previews"
)

// RunContext is the context for a run
type RunContext struct {
	StackID string
	Cmd     []string
}

// CreatePreviewOpts is the options for the CreatePreview function
type CreatePreviewOpts struct {
	Runs            []RunContext
	AffectedStacks  map[string]resources.Stack
	OrgUUID         resources.UUID
	PushedAt        int64
	CommitSHA       string
	Technology      string
	TechnologyLayer string
	ReviewRequest   *resources.ReviewRequest
	Metadata        *resources.DeploymentMetadata
}

// CreatedPreview is the result of CreatePreview
type CreatedPreview struct {
	ID                    string
	StackPreviewsByMetaID map[string]string
}

// UpdateStackPreviewOpts is the options for UpdateStackPreview
type UpdateStackPreviewOpts struct {
	OrgUUID          resources.UUID
	StackPreviewID   string
	Status           preview.StackStatus
	ChangesetDetails *resources.ChangesetDetails
}

// UpdateStackPreview updates a stack preview in the cloud.
func (c *Client) UpdateStackPreview(ctx context.Context, opts UpdateStackPreviewOpts) error {
	if err := opts.validate(); err != nil {
		return err
	}
	payload := resources.UpdateStackPreviewPayloadRequest{
		Status: opts.Status.String(),
	}
	if opts.ChangesetDetails != nil {
		payload.ChangesetDetails = &resources.ChangesetDetails{
			Provisioner:    opts.ChangesetDetails.Provisioner,
			ChangesetASCII: opts.ChangesetDetails.ChangesetASCII,
			ChangesetJSON:  opts.ChangesetDetails.ChangesetJSON,
		}
	}

	return c.updateStackPreview(ctx, opts.OrgUUID, opts.StackPreviewID, payload)
}

// CreatePreview creates a new preview in the cloud
func (c *Client) CreatePreview(ctx context.Context, opts CreatePreviewOpts) (*CreatedPreview, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	payload := resources.CreatePreviewPayloadRequest{
		PushedAt:        opts.PushedAt,
		CommitSHA:       opts.CommitSHA,
		Technology:      opts.Technology,
		TechnologyLayer: opts.TechnologyLayer,
		ReviewRequest:   opts.ReviewRequest,
		Metadata:        opts.Metadata,
		Stacks:          []resources.PreviewStack{},
	}

	previewStacksMap := map[string]RunContext{}
	for _, run := range opts.Runs {
		previewStacksMap[run.StackID] = run
	}

	// loop over all affected stacks, if an item is present in the
	// previewStacksMap, use the preview status and cmd from there
	for _, affectedStack := range opts.AffectedStacks {
		stack := resources.PreviewStack{
			PreviewStatus: preview.StackStatusAffected,
			Cmd:           []string{},
			Stack:         affectedStack,
		}

		if previewStack, found := previewStacksMap[affectedStack.MetaID]; found {
			stack.PreviewStatus = preview.StackStatusPending
			stack.Cmd = previewStack.Cmd
		}
		payload.Stacks = append(payload.Stacks, stack)
	}

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

// createPreview creates a new preview for an organization
func (c *Client) createPreview(
	ctx context.Context,
	orgUUID resources.UUID,
	payload resources.CreatePreviewPayloadRequest,
) (resources.CreatePreviewResponse, error) {
	if err := payload.Validate(); err != nil {
		return resources.CreatePreviewResponse{}, errors.E(err, "invalid payload")
	}

	return http.Post[resources.CreatePreviewResponse](
		ctx, c, payload,
		c.URL(path.Join(PreviewsPath, string(orgUUID))),
	)
}

// updateStackPreview updates a stack preview for an organization
func (c *Client) updateStackPreview(
	ctx context.Context,
	orgUUID resources.UUID,
	stackPreviewID string,
	payload resources.UpdateStackPreviewPayloadRequest,
) error {
	if err := payload.Validate(); err != nil {
		return errors.E(err, "invalid payload")
	}

	// Endpoint: /v1/stack_previews/{org_uuid}/{stack_preview_id}
	_, err := http.Patch[resources.EmptyResponse](
		ctx, c, payload,
		c.URL(path.Join(StackPreviewsPath, string(orgUUID), stackPreviewID)),
	)
	return err
}

func (o CreatePreviewOpts) validate() error {
	errs := errors.L()

	if string(o.OrgUUID) == "" {
		errs.Append(errors.E("org uuid is empty"))
	}

	return errs.AsError()
}

func (o UpdateStackPreviewOpts) validate() error {
	errs := errors.L()
	if o.StackPreviewID == "" {
		errs.Append(errors.E("stack preview id is empty"))
	}

	if string(o.OrgUUID) == "" {
		errs.Append(errors.E("org uuid is empty"))
	}

	if err := o.Status.Validate(); err != nil {
		errs.Append(err)
	}

	if o.ChangesetDetails != nil {
		if err := o.ChangesetDetails.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	"path"
	"strings"

	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

const (
	// PreviewsPath is the previews endpoints base path.
	PreviewsPath = "/v1/previews"
	// StackPreviewsPath is the stack previews endpoint base path.
	StackPreviewsPath = "/v1/stack_previews"
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
	PushedAt        int64
	CommitSHA       string
	Technology      string
	TechnologyLayer string
	Repository      string
	DefaultBranch   string
	ReviewRequest   *ReviewRequest
	Metadata        *DeploymentMetadata
}

// CreatedPreview is the result of CreatePreview
type CreatedPreview struct {
	ID                    string
	StackPreviewsByMetaID map[string]string
}

// UpdateStackPreviewOpts is the options for UpdateStackPreview
type UpdateStackPreviewOpts struct {
	OrgUUID          UUID
	StackPreviewID   string
	Status           preview.StackStatus
	ChangesetDetails *ChangesetDetails
}

// UpdateStackPreview updates a stack preview in the cloud.
func (c *Client) UpdateStackPreview(ctx context.Context, opts UpdateStackPreviewOpts) error {
	if err := opts.validate(); err != nil {
		return err
	}
	payload := UpdateStackPreviewPayloadRequest{
		Status: opts.Status.String(),
	}
	if opts.ChangesetDetails != nil {
		payload.ChangesetDetails = &ChangesetDetails{
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

	payload := CreatePreviewPayloadRequest{
		UpdatedAt:       opts.UpdatedAt,
		PushedAt:        opts.PushedAt,
		CommitSHA:       opts.CommitSHA,
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
			PreviewStatus: preview.StackStatusAffected,
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
	orgUUID UUID,
	payload CreatePreviewPayloadRequest,
) (CreatePreviewResponse, error) {
	if err := payload.Validate(); err != nil {
		return CreatePreviewResponse{}, errors.E(err, "invalid payload")
	}

	return Post[CreatePreviewResponse](
		ctx, c, payload,
		c.URL(path.Join(PreviewsPath, string(orgUUID))),
	)
}

// updateStackPreview updates a stack preview for an organization
func (c *Client) updateStackPreview(
	ctx context.Context,
	orgUUID UUID,
	stackPreviewID string,
	payload UpdateStackPreviewPayloadRequest,
) error {
	if err := payload.Validate(); err != nil {
		return errors.E(err, "invalid payload")
	}

	// Endpoint: /v1/stack_previews/{org_uuid}/{stack_preview_id}
	_, err := Patch[EmptyResponse](
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

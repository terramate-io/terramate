// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package previews

import (
	"context"
	"strings"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

const (
	// PreviewStatusAffected is the status for a stack that is affected in a PR
	PreviewStatusAffected = "affected"
	// PreviewStatusPending is the status for a stack that is selected in a preview run
	PreviewStatusPending = "pending"
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
	OrgUUID         cloud.UUID
	UpdatedAt       int64
	Technology      string
	TechnologyLayer string
	Repository      string
	DefaultBranch   string
	ReviewRequest   *cloud.DeploymentReviewRequest
	Metadata        *cloud.DeploymentMetadata
}

// CreatedPreview is the result of the CreatePreview function
type CreatedPreview struct {
	ID                    string
	StackPreviewsByMetaID map[string]string
}

// ChangesetDetails is the details of a changeset
type ChangesetDetails struct {
	Provisioner    string
	ChangesetASCII string
	ChangesetJSON  string
}

// CreateStackPreviewOpts is the options for the CreateStackPreview function
type CreateStackPreviewOpts struct {
	OrgUUID          cloud.UUID
	StackPreviewID   string
	ChangesetDetails *ChangesetDetails
}

// StackPreviewLogLine is a log line for a stack preview
type StackPreviewLogLine struct {
	LogLine   string
	Timestamp time.Time
	Channel   string
	Message   string
}

// AppendStackPreviewLogsOpts is the options for the AppendStackPreviewLogs function
type AppendStackPreviewLogsOpts struct {
	OrgUUID        cloud.UUID
	StackPreviewID string
	Logs           []StackPreviewLogLine
}

// AppendStackPreviewLogs appends logs to a stack preview in the cloud
// nolint:revive
func AppendStackPreviewLogs(client *cloud.Client, timeout time.Duration, opts AppendStackPreviewLogsOpts) (map[string]string, error) {
	return nil, errors.E("not implemented")
}

// CreateStackPreview creates a new stack preview in the cloud
// nolint:revive
func CreateStackPreview(client *cloud.Client, timeout time.Duration, opts CreateStackPreviewOpts) (map[string]string, error) {
	return nil, errors.E("not implemented")
}

// CreatePreview creates a new preview in the cloud
func CreatePreview(client *cloud.Client, timeout time.Duration, opts CreatePreviewOpts) (*CreatedPreview, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	payload := cloud.CreatePreviewPayloadRequest{
		UpdatedAt:       opts.UpdatedAt,
		Technology:      opts.Technology,
		TechnologyLayer: opts.TechnologyLayer,
		ReviewRequest:   opts.ReviewRequest,
		Metadata:        opts.Metadata,
		Stacks:          []cloud.PreviewStack{},
	}

	previewStacksMap := map[string]RunContext{}
	for _, run := range opts.Runs {
		previewStacksMap[run.Stack.ID] = run
	}

	// loop over all affected stacks, if an item is present in the
	// previewStacksMap, use the preview status and cmd from there
	for _, affectedStack := range opts.AffectedStacks {
		tags := []string{}
		if len(affectedStack.Tags) > 0 {
			tags = affectedStack.Tags
		}

		stack := cloud.PreviewStack{
			PreviewStatus:   PreviewStatusAffected,
			Repository:      opts.Repository,
			Path:            affectedStack.Dir.String(),
			MetaID:          strings.ToLower(affectedStack.ID),
			MetaName:        affectedStack.Name,
			MetaDescription: affectedStack.Description,
			MetaTags:        tags,
			DefaultBranch:   opts.DefaultBranch,
			Cmd:             []string{},
		}

		if previewStack, found := previewStacksMap[affectedStack.ID]; found {
			stack.PreviewStatus = PreviewStatusPending
			stack.Cmd = previewStack.Cmd
		}
		payload.Stacks = append(payload.Stacks, stack)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := client.CreatePreview(ctx, opts.OrgUUID, payload)
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

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

// CreateCloudPreview creates a cloud preview for the given runs.
func CreateCloudPreview(e *engine.Engine, gitfilter engine.GitFilter, runs []engine.StackCloudRun, target, fromTarget string, state *CloudRunState) map[string]string {
	previewRuns := make([]cloud.RunContext, len(runs))
	for i, run := range runs {
		previewRuns[i] = cloud.RunContext{
			StackID: run.Stack.ID,
			Cmd:     run.Task.Cmd,
		}
	}

	prj := e.Project()
	affectedStacksMap := map[string]resources.Stack{}
	affectedStacks, err := e.GetAffectedStacks(gitfilter)
	if err != nil {
		e.DisableCloudFeatures(err)
		return map[string]string{}
	}
	prettyRepo, err := prj.PrettyRepo()
	if err != nil {
		e.DisableCloudFeatures(err)
		return map[string]string{}
	}

	for _, st := range affectedStacks {
		affectedStacksMap[st.Stack.ID] = resources.Stack{
			Path:            st.Stack.Dir.String(),
			MetaID:          strings.ToLower(st.Stack.ID),
			MetaName:        st.Stack.Name,
			MetaDescription: st.Stack.Description,
			MetaTags:        st.Stack.Tags,
			Repository:      prettyRepo,
			Target:          target,
			FromTarget:      fromTarget,
			DefaultBranch:   prj.GitConfig().DefaultBranch,
		}
	}

	if state.ReviewRequest == nil || state.RREvent.PushedAt == nil {
		printer.Stderr.WarnWithDetails(
			"unable to create preview: missing review request information",
			errors.E("--sync-preview can only be used when GITHUB_TOKEN or GITLAB_TOKEN is exported and Terramate runs in a CI/CD environment triggered by a Pull/Merge Request event"),
		)
		e.DisableCloudFeatures(cloudError())
		return map[string]string{}
	}

	state.ReviewRequest.PushedAt = state.RREvent.PushedAt

	// preview always requires a commit_sha, so if the API failed to provide it, we should give the HEAD commit.
	if state.RREvent.CommitSHA == "" {
		state.RREvent.CommitSHA, err = prj.HeadCommit()
		if err != nil {
			e.DisableCloudFeatures(err)
			return map[string]string{}
		}
	}

	technology := "other"
	technologyLayer := "default"
	for _, run := range runs {
		if run.Task.CloudPlanFile != "" {
			technology = run.Task.CloudPlanProvisioner
		}
		if layer := run.Task.CloudSyncLayer; layer != "" {
			technologyLayer = fmt.Sprintf("custom:%s", layer)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	createdPreview, err := e.CloudClient().CreatePreview(
		ctx,
		cloud.CreatePreviewOpts{
			Runs:            previewRuns,
			AffectedStacks:  affectedStacksMap,
			OrgUUID:         e.CloudState().Org.UUID,
			PushedAt:        *state.RREvent.PushedAt,
			CommitSHA:       state.RREvent.CommitSHA,
			Technology:      technology,
			TechnologyLayer: technologyLayer,
			ReviewRequest:   state.ReviewRequest,
			Metadata:        state.Metadata,
		},
	)
	if err != nil {
		printer.Stderr.WarnWithDetails("unable to create preview", err)
		e.DisableCloudFeatures(cloudError())
		return map[string]string{}
	}

	printer.Stderr.Success(fmt.Sprintf("Preview created (id: %s)", createdPreview.ID))

	return createdPreview.StackPreviewsByMetaID
}

func doPreviewBefore(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState) {
	stackPreviewID, ok := state.CloudPreviewID(run.Stack.ID)
	if !ok {
		e.DisableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	if err := e.CloudClient().UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          e.CloudState().Org.UUID,
			StackPreviewID:   stackPreviewID,
			Status:           preview.StackStatusRunning,
			ChangesetDetails: nil,
		}); err != nil {
		printer.Stderr.ErrorWithDetails("failed to update stack preview", err)
		return
	}
	log.Debug().
		Str("stack_name", run.Stack.Dir.String()).
		Str("stack_preview_status", preview.StackStatusRunning.String()).
		Msg("Setting stack preview status")
}

func doPreviewAfter(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, res engine.RunResult, _ error) {
	planfile := run.Task.CloudPlanFile

	previewStatus := preview.DerivePreviewStatus(res.ExitCode)
	var previewChangeset *resources.ChangesetDetails
	if planfile != "" && previewStatus != preview.StackStatusCanceled {
		changeset, err := getTerraformChangeset(e, run)
		if err != nil || changeset == nil {
			printer.Stderr.WarnWithDetails(
				fmt.Sprintf("skipping terraform plan sync for %s", run.Stack.Dir.String()), err,
			)

			if previewStatus != preview.StackStatusFailed {
				printer.Stderr.Warn(
					fmt.Sprintf("preview status set to \"failed\" (previously %q) due to failure when generating the "+
						"changeset details", previewStatus),
				)

				previewStatus = preview.StackStatusFailed
			}
		}
		if changeset != nil {
			previewChangeset = &resources.ChangesetDetails{
				Provisioner:    changeset.Provisioner,
				ChangesetASCII: changeset.ChangesetASCII,
				ChangesetJSON:  changeset.ChangesetJSON,
			}
		}
	}

	stackPreviewID, ok := state.CloudPreviewID(run.Stack.ID)
	if !ok {
		e.DisableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	if err := e.CloudClient().UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          e.CloudState().Org.UUID,
			StackPreviewID:   stackPreviewID,
			Status:           previewStatus,
			ChangesetDetails: previewChangeset,
		}); err != nil {
		printer.Stderr.ErrorWithDetails("failed to create stack preview", err)
		return
	}

	logger := log.With().
		Str("stack_name", run.Stack.Dir.String()).
		Str("stack_preview_status", previewStatus.String()).
		Logger()

	logger.Debug().Msg("Setting stack preview status")
	if previewChangeset != nil {
		logger.Debug().Msg("Sending changelog")
	}
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	tel "github.com/terramate-io/terramate/cmd/terramate/cli/telemetry"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/printer"
)

const (
	defaultCloudTimeout     = 60 * time.Second
	defaultGoogleTimeout    = defaultCloudTimeout
	defaultGithubTimeout    = defaultCloudTimeout
	defaultGitlabTimeout    = defaultCloudTimeout
	defaultBitbucketTimeout = defaultCloudTimeout
)

type cloudState struct {
	disabled      bool
	client        *cloud.Client
	stdoutPrinter *printer.Printer

	run cloudRunState
}

type cloudRunState struct {
	runUUID cloud.UUID
	orgName string
	orgUUID cloud.UUID
	target  string

	stackMeta2ID map[string]int64
	// stackPreviews is a map of stack.ID to stackPreview.ID
	stackMeta2PreviewIDs map[string]string
	reviewRequest        *cloud.ReviewRequest
	rrEvent              struct {
		pushedAt  *int64
		commitSHA string
	}
	metadata *cloud.DeploymentMetadata
}

type cloudConfig struct {
	disabled bool
	client   *cloud.Client

	run cloudRunState
}

func (rs *cloudRunState) setMeta2CloudID(metaID string, id int64) {
	if rs.stackMeta2ID == nil {
		rs.stackMeta2ID = make(map[string]int64)
	}
	rs.stackMeta2ID[strings.ToLower(metaID)] = id
}

func (rs cloudRunState) stackCloudID(metaID string) (int64, bool) {
	id, ok := rs.stackMeta2ID[strings.ToLower(metaID)]
	return id, ok
}

func (rs *cloudRunState) setMeta2PreviewID(metaID string, previewID string) {
	if rs.stackMeta2PreviewIDs == nil {
		rs.stackMeta2PreviewIDs = make(map[string]string)
	}
	rs.stackMeta2PreviewIDs[strings.ToLower(metaID)] = previewID
}

func (rs cloudRunState) cloudPreviewID(metaID string) (string, bool) {
	id, ok := rs.stackMeta2PreviewIDs[strings.ToLower(metaID)]
	return id, ok
}

// newCloudLoginRequiredError creates an error indicating that a cloud login is required to use requested features.
func newCloudLoginRequiredError(requestedFeatures []string) *errors.DetailedError {
	err := errors.D(clitest.CloudLoginRequiredMessage)

	for _, s := range requestedFeatures {
		err = err.WithDetailf(verbosity.V1, "%s", s)
	}

	err = err.WithDetailf(verbosity.V1, "To login with an existing account, run 'terramate cloud login'.").
		WithDetailf(verbosity.V1, "To create a free account, visit https://cloud.terramate.io.")

	return err.WithCode(clitest.ErrCloud)
}

func newCloudOnboardingIncompleteError(region cloud.Region) *errors.DetailedError {
	err := errors.D(clitest.CloudOnboardingIncompleteMessage)
	err = err.WithDetailf(verbosity.V1, "Visit %s to setup your account.", cloud.HTMLURL(region))
	return err.WithCode(clitest.ErrCloudOnboardingIncomplete)
}

func (e *Engine) loadCredential() error {
	region := e.cloudRegion()
	cloudURL, envFound := tmcloud.EnvBaseURL()
	if !envFound {
		cloudURL = cloud.BaseURL(region)
	}
	clientLogger := log.With().
		Str("tmc_url", cloudURL).
		Logger()

	e.state.cloud.client = &cloud.Client{
		Region:     region, // always set so we can use it in error messages
		HTTPClient: &e.httpClient,
		Logger:     &clientLogger,
	}
	if envFound {
		e.state.cloud.client.BaseURL = cloudURL
	}

	// checks if this client version can communicate with Terramate Cloud.
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	err := e.state.cloud.client.CheckVersion(ctx)
	if err != nil {
		return errors.E(err, clitest.ErrCloudCompat)
	}

	probes := auth.ProbingPrecedence(e.printers, e.state.cloud.client, e.usercfg)
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return err
		}
		if found {
			break
		}
	}
	if !found {
		return errors.E("no credential found", auth.ErrLoginRequired)
	}
	return nil
}

func (e *Engine) SetupCloudConfig(requestedFeatures []string) error {
	if e.state.cloud.run.orgUUID != "" {
		// already setup
		return nil
	}
	err := e.loadCredential()
	if err != nil {
		if errors.IsKind(err, auth.ErrLoginRequired) {
			return newCloudLoginRequiredError(requestedFeatures).WithCause(err)
		}
		if errors.IsKind(err, clitest.ErrCloudOnboardingIncomplete) {
			return newCloudOnboardingIncompleteError(e.state.cloud.client.Region).WithCause(err)
		}
		printer.Stderr.ErrorWithDetails("failed to load the cloud credentials", err)
		return cloudError()
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := e.cred().Organizations()

	useOrgName := e.cloudOrgName()
	e.state.cloud.run.orgName = useOrgName
	if useOrgName != "" {
		var useOrgUUID cloud.UUID
		for _, org := range orgs {
			if strings.EqualFold(org.Name, useOrgName) {
				if org.Status != "active" && org.Status != "trusted" {
					printer.Stderr.ErrorWithDetails(
						"Invalid membership status",
						errors.E(
							"You are not yet an active member of organization %s. Please accept the invitation first.",
							useOrgName,
						),
					)

					return cloudError()
				}

				useOrgUUID = org.UUID
				break
			}
		}

		if useOrgUUID == "" {
			printer.Stderr.ErrorWithDetails(
				"Invalid membership status",
				errors.E(
					"You are not a member of organization %q or the organization does not exist. Available organizations: %s",
					useOrgName,
					orgs,
				),
			)

			return cloudError()
		}

		e.state.cloud.run.orgUUID = useOrgUUID
	} else {
		var activeOrgs cloud.MemberOrganizations
		var invitedOrgs cloud.MemberOrganizations
		for _, org := range orgs {
			if org.Status == "active" || org.Status == "trusted" {
				activeOrgs = append(activeOrgs, org)
			} else if org.Status == "invited" {
				invitedOrgs = append(invitedOrgs, org)
			}
		}
		if len(activeOrgs) == 0 {
			printer.Stderr.Error(clitest.CloudNoMembershipMessage)

			if len(invitedOrgs) > 0 {
				printer.Stderr.WarnWithDetails(
					"Pending invitation",
					errors.E(
						"You have pending invitation for the following organizations: %s",
						invitedOrgs,
					),
				)
			}

			return errors.E(clitest.ErrCloudOnboardingIncomplete)
		}
		printer.Stderr.ErrorWithDetails(
			"Missing cloud configuration",
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
				"terramate.config.cloud.organization configuration attribute to a specific organization",
			),
		)
		return cloudError()
	}

	tel.DefaultRecord.Set(tel.OrgUUID(e.state.cloud.run.orgUUID))
	return nil
}

func (e *Engine) cloudOrgName() string {
	orgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if orgName != "" {
		return orgName
	}
	cfg := e.RootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Cloud != nil {
		return cfg.Terramate.Config.Cloud.Organization
	}
	return ""
}

func (c *Engine) IsCloudEnabled() bool {
	return !c.state.cloud.disabled
}

func (c *Engine) disableCloudFeatures(err error) {
	printer.Stderr.WarnWithDetails(clitest.CloudDisablingMessage, errors.E(err.Error()))

	c.state.cloud.disabled = true
}

func (e *Engine) cloudSyncBefore(run stackCloudRun) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		e.doCloudSyncDeployment(run, deployment.Running)
	}

	if run.Task.CloudSyncPreview {
		e.doPreviewBefore(run)
	}
}

func (e *Engine) cloudSyncAfter(run stackCloudRun, res runResult, err error) {
	if !e.IsCloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		e.cloudSyncDeployment(run, err)
	}

	if run.Task.CloudSyncDriftStatus {
		e.cloudSyncDriftStatus(run, res, err)
	}

	if run.Task.CloudSyncPreview {
		e.doPreviewAfter(run, res)
	}
}

func (e *Engine) doPreviewBefore(run stackCloudRun) {
	stackPreviewID, ok := e.state.cloud.run.cloudPreviewID(run.Stack.ID)
	if !ok {
		e.disableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	if err := e.state.cloud.client.UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          e.state.cloud.run.orgUUID,
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

func (e *Engine) doPreviewAfter(run stackCloudRun, res runResult) {
	planfile := run.Task.CloudPlanFile

	previewStatus := preview.DerivePreviewStatus(res.ExitCode)
	var previewChangeset *cloud.ChangesetDetails
	if planfile != "" && previewStatus != preview.StackStatusCanceled {
		changeset, err := e.getTerraformChangeset(run)
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
			previewChangeset = &cloud.ChangesetDetails{
				Provisioner:    changeset.Provisioner,
				ChangesetASCII: changeset.ChangesetASCII,
				ChangesetJSON:  changeset.ChangesetJSON,
			}
		}
	}

	stackPreviewID, ok := e.state.cloud.run.cloudPreviewID(run.Stack.ID)
	if !ok {
		e.disableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	if err := e.state.cloud.client.UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          e.state.cloud.run.orgUUID,
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

func cloudError() error {
	return errors.E(clitest.ErrCloud)
}

func (e *Engine) HandleCloudCriticalError(err error) error {
	if err != nil {
		if e.uimode == HumanMode {
			return err
		}

		e.disableCloudFeatures(err)
	}
	return nil
}

func (e *Engine) IsCloudDisabled() bool {
	return !e.IsCloudEnabled()
}

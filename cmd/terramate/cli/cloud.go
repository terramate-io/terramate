// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdjson "encoding/json"
	stdfmt "fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/golang-jwt/jwt"
	"github.com/google/go-github/v58/github"
	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog/log"
	githubql "github.com/shurcooL/githubv4"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	tmgithub "github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/gitlab"

	"golang.org/x/oauth2"

	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
)

const (
	defaultCloudTimeout  = 60 * time.Second
	defaultGoogleTimeout = defaultCloudTimeout
	defaultGithubTimeout = defaultCloudTimeout
	defaultGitlabTimeout = defaultCloudTimeout
)

const (
	cloudFeatStatus          = "--status' is a Terramate Cloud feature to filter stacks that failed to deploy or have drifted."
	cloudFeatSyncDeployment  = "'--sync-deployment' is a Terramate Cloud feature to synchronize deployment details to Terramate Cloud."
	cloudFeatSyncDriftStatus = "'--sync-drift-status' is a Terramate Cloud feature to synchronize drift and health check results to Terramate Cloud."
	cloudFeatSyncPreview     = "'--sync-preview' is a Terramate Cloud feature to synchronize deployment previews to Terramate Cloud."
)

const githubDomain = "github.com"
const gitlabDomain = "gitlab.com"

const (
	githubErrNotFound            errors.Kind = "resource not found (HTTP Status: 404)"
	githubErrUnprocessableEntity errors.Kind = "entity cannot be processed (HTTP Status: 422)"

	// ErrLoginRequired is an error indicating that user has to login to the cloud.
	ErrLoginRequired errors.Kind = "cloud login required"

	// ErrIDPNeedConfirmation is an error indicating the user has multiple providers set up and
	// linking them is needed.
	ErrIDPNeedConfirmation errors.Kind = "the account was already set up with another email provider"

	// ErrEmailNotVerified is an error indicating that user's email need to be verified.
	ErrEmailNotVerified errors.Kind = "email is not verified"
)

// newCloudRequiredError creates an error indicating that a cloud login is required to use requested features.
func newCloudRequiredError(requestedFeatures []string) *errors.DetailedError {
	err := errors.D(clitest.CloudLoginRequiredMessage)

	for _, s := range requestedFeatures {
		err = err.WithDetailf(verbosity.V1, "%s", s)
	}

	err = err.WithDetailf(verbosity.V1, "To login with an existing account, run 'terramate cloud login'.").
		WithDetailf(verbosity.V1, "To create a free account, visit https://cloud.terramate.io.")

	return err.WithCode(clitest.ErrCloud)
}

// newIDPNeedConfirmationError creates an error indicating the user has multiple providers set up and
// linking them is needed.
func newIDPNeedConfirmationError(verifiedProviders []string) *errors.DetailedError {
	err := errors.D("The account was already set up with another email provider.")

	if len(verifiedProviders) > 0 {
		err = err.WithDetailf(verbosity.V1, "Please login using one of the methods below:")
		for _, providerDomain := range verifiedProviders {
			switch providerDomain {
			case "google.com":
				err = err.WithDetailf(verbosity.V1, "- Run 'terramate cloud login --google' to login with your Google account")
			case "github.com":
				err = err.WithDetailf(verbosity.V1, "- Run 'terramate cloud login --github' to login with your GitHub account")
			}
			err = err.WithDetailf(verbosity.V1, "Alternatively, visit https://cloud.terramate.io and authenticate with the Social login to link the accounts.")
		}
	} else {
		err = err.WithDetailf(verbosity.V1, "Visit https://cloud.terramate.io and authenticate to link the accounts.")
	}

	return err.WithCode(ErrIDPNeedConfirmation)
}

// newEmailNotVerifiedError creates an error indicating that user's email need to be verified.
func newEmailNotVerifiedError(email string) *errors.DetailedError {
	return errors.D("Email %s is not verified.", email).
		WithDetailf(verbosity.V1, "Please login to https://cloud.terramate.io to verify your email and continue the sign up process.").
		WithCode(ErrEmailNotVerified)
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
		pushedAt  *time.Time
		commitSHA string
	}
	metadata *cloud.DeploymentMetadata
}

type cloudConfig struct {
	disabled bool
	client   *cloud.Client
	output   out.O

	run cloudRunState
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	IsExpired() bool
	ExpireAt() time.Time

	// private interface

	organizations() cloud.MemberOrganizations
	info(selectedOrgName string)
}

type keyValue struct {
	key   string
	value string
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

func (c *cli) credentialPrecedence(output out.O) []credential {
	return []credential{
		newGithubOIDC(output, c.cloud.client),
		newGitlabOIDC(output, c.cloud.client),
		newGoogleCredential(output, c.cloud.client.IDPKey, c.clicfg, c.cloud.client),
	}
}

func (c *cli) cloudEnabled() bool {
	return !c.cloud.disabled
}

func (c *cli) disableCloudFeatures(err error) {
	printer.Stderr.WarnWithDetails(clitest.CloudDisablingMessage, errors.E(err.Error()))

	c.cloud.disabled = true
}

func (c *cli) handleCriticalError(err error) {
	if err != nil {
		if c.uimode == HumanMode {
			fatal(err)
		}

		c.disableCloudFeatures(err)
	}
}

func selectCloudStackTasks(runs []stackRun, pred func(stackRunTask) bool) []stackCloudRun {
	var cloudRuns []stackCloudRun
	for _, run := range runs {
		for _, t := range run.Tasks {
			if pred(t) {
				cloudRuns = append(cloudRuns, stackCloudRun{
					Stack: run.Stack,
					Task:  t,
				})
				// Currently, only a single task per stackRun group may be selected.
				break
			}
		}
	}
	return cloudRuns
}

func isDeploymentTask(t stackRunTask) bool { return t.CloudSyncDeployment }
func isDriftTask(t stackRunTask) bool      { return t.CloudSyncDriftStatus }
func isPreviewTask(t stackRunTask) bool    { return t.CloudSyncPreview }

func (c *cli) checkCloudSync() {
	if !c.parsedArgs.Run.SyncDeployment && !c.parsedArgs.Run.SyncDriftStatus && !c.parsedArgs.Run.SyncPreview {
		return
	}

	var feats []string
	if c.parsedArgs.Run.SyncDeployment {
		feats = append(feats, cloudFeatSyncDeployment)
	}
	if c.parsedArgs.Run.SyncDriftStatus {
		feats = append(feats, cloudFeatSyncDriftStatus)
	}
	if c.parsedArgs.Run.SyncPreview {
		feats = append(feats, cloudFeatSyncPreview)
	}

	err := c.setupCloudConfig(feats)
	c.handleCriticalError(err)

	if c.cloud.disabled {
		return
	}

	if c.parsedArgs.Run.SyncDeployment {
		uuid, err := uuid.GenerateUUID()
		c.handleCriticalError(err)
		c.cloud.run.runUUID = cloud.UUID(uuid)
	}
}

func (c *cli) cloudOrgName() string {
	orgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if orgName != "" {
		return orgName
	}

	cfg := c.rootNode()
	if cfg.Terramate != nil &&
		cfg.Terramate.Config != nil &&
		cfg.Terramate.Config.Cloud != nil {
		return cfg.Terramate.Config.Cloud.Organization
	}

	return ""
}

func (c *cli) setupCloudConfig(requestedFeatures []string) error {
	err := c.loadCredential()
	if err != nil {
		if errors.IsKind(err, ErrLoginRequired) {
			return newCloudRequiredError(requestedFeatures).WithCause(err)
		}
		printer.Stderr.ErrorWithDetails("failed to load the cloud credentials", err)
		return cloudError()
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := c.cred().organizations()

	useOrgName := c.cloudOrgName()
	c.cloud.run.orgName = useOrgName
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

		c.cloud.run.orgUUID = useOrgUUID
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
		if len(activeOrgs) > 1 {
			printer.Stderr.ErrorWithDetails(
				"Invalid cloud configuration",
				errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
					"terramate.config.cloud.organization configuration attribute to a specific available organization: %s",
					activeOrgs,
				),
			)
			return cloudError()
		}

		c.cloud.run.orgName = activeOrgs[0].Name
		c.cloud.run.orgUUID = activeOrgs[0].UUID
	}
	return nil
}

func (c *cli) cloudSyncBefore(run stackCloudRun) {
	if !c.cloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		c.doCloudSyncDeployment(run, deployment.Running)
	}

	if run.Task.CloudSyncPreview {
		c.doPreviewBefore(run)
	}
}

func (c *cli) cloudSyncAfter(run stackCloudRun, res runResult, err error) {
	if !c.cloudEnabled() {
		return
	}

	if run.Task.CloudSyncDeployment {
		c.cloudSyncDeployment(run, err)
	}

	if run.Task.CloudSyncDriftStatus {
		c.cloudSyncDriftStatus(run, res, err)
	}

	if run.Task.CloudSyncPreview {
		c.doPreviewAfter(run, res)
	}
}

func (c *cli) doPreviewBefore(run stackCloudRun) {
	stackPreviewID, ok := c.cloud.run.cloudPreviewID(run.Stack.ID)
	if !ok {
		c.disableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	if err := c.cloud.client.UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          c.cloud.run.orgUUID,
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

func (c *cli) doPreviewAfter(run stackCloudRun, res runResult) {
	planfile := run.Task.CloudPlanFile

	previewStatus := preview.DerivePreviewStatus(res.ExitCode)
	var previewChangeset *cloud.ChangesetDetails
	if planfile != "" && previewStatus != preview.StackStatusCanceled {
		changeset, err := c.getTerraformChangeset(run)
		if err != nil || changeset == nil {
			printer.Stderr.WarnWithDetails(
				stdfmt.Sprintf("skipping terraform plan sync for %s", run.Stack.Dir.String()), err,
			)

			if previewStatus != preview.StackStatusFailed {
				printer.Stderr.Warn(
					stdfmt.Sprintf("preview status set to \"failed\" (previously %q) due to failure when generating the "+
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

	stackPreviewID, ok := c.cloud.run.cloudPreviewID(run.Stack.ID)
	if !ok {
		c.disableCloudFeatures(errors.E(errors.ErrInternal, "failed to get previewID"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	if err := c.cloud.client.UpdateStackPreview(ctx,
		cloud.UpdateStackPreviewOpts{
			OrgUUID:          c.cloud.run.orgUUID,
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

func (c *cli) cloudInfo() {
	err := c.loadCredential()
	if err != nil {
		// TODO: Better error message.
		fatalWithDetailf(err, "failed to load credentials")
	}
	c.cred().info(c.cloudOrgName())

	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
}

func (c *cli) cloudDriftShow() {
	err := c.setupCloudConfig(nil)
	if err != nil {
		fatalWithDetailf(err, "unable to setup cloud configuration")
	}
	st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
	if err != nil {
		fatalWithDetailf(err, "loading stack in current directory")
	}
	if !found {
		fatal("No stack selected. Please enter a stack to show a potential drift.")
	}
	if st.ID == "" {
		fatal("The stack must have an ID for using TMC features")
	}

	target := c.parsedArgs.Cloud.Drift.Show.Target

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	isTargetConfigEnabled := false
	c.checkTargetsConfiguration(target, "", func(isTargetEnabled bool) {
		if !isTargetEnabled {
			fatal("--target must be set when terramate.config.cloud.targets.enabled is true")
		}
		isTargetConfigEnabled = isTargetEnabled
	})

	if target == "" {
		target = "default"
	}

	stackResp, found, err := c.cloud.client.GetStack(ctx, c.cloud.run.orgUUID, c.prj.prettyRepo(), target, st.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch stack")
	}
	if !found {
		if isTargetConfigEnabled {
			fatalf("Stack %s was not yet synced for target %s with the Terramate Cloud.", st.Dir.String(), target)
		} else {
			fatalf("Stack %s was not yet synced with the Terramate Cloud.", st.Dir.String())
		}
	}

	if stackResp.Status != stack.Drifted && stackResp.DriftStatus != drift.Drifted {
		c.output.MsgStdOut("Stack %s is not drifted.", st.Dir.String())
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	// stack is drifted
	driftsResp, err := c.cloud.client.StackLastDrift(ctx, c.cloud.run.orgUUID, stackResp.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch drift")
	}
	if len(driftsResp.Drifts) == 0 {
		fatalf("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	driftData := driftsResp.Drifts[0]

	ctx, cancel = context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	driftData, err = c.cloud.client.DriftDetails(ctx, c.cloud.run.orgUUID, stackResp.ID, driftData.ID)
	if err != nil {
		fatalWithDetailf(err, "unable to fetch drift details")
	}
	if driftData.Status != drift.Drifted || driftData.Details == nil || driftData.Details.Provisioner == "" {
		fatalf("Stack %s is drifted, but no details are available.", st.Dir.String())
	}
	c.output.MsgStdOutV("drift provisioner: %s", driftData.Details.Provisioner)
	c.output.MsgStdOut(driftData.Details.ChangesetASCII)
}

func (c *cli) detectCloudMetadata() {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Str("action", "detectCloudMetadata").
		Logger()

	prettyRepo := c.prj.prettyRepo()
	if prettyRepo == "local" || c.prj.repository == nil {
		printer.Stderr.Warn(errors.E("skipping review_request and remote metadata for local repository"))
		return
	}

	c.cloud.run.metadata = &cloud.DeploymentMetadata{}
	c.cloud.run.metadata.GitCommitSHA = c.prj.headCommit()

	md := c.cloud.run.metadata

	defer func() {
		if c.cloud.run.metadata != nil {
			data, err := stdjson.Marshal(c.cloud.run.metadata)
			if err == nil {
				logger.Debug().RawJSON("provider_metadata", data).Msg("detected metadata")
			} else {
				logger.Warn().Err(err).Msg("failed to encode deployment metadata")
			}
		} else {
			logger.Debug().Msg("no provider metadata detected")
		}

		if c.cloud.run.reviewRequest != nil {
			data, err := stdjson.Marshal(c.cloud.run.reviewRequest)
			if err == nil {
				logger.Debug().RawJSON("provider_review_request", data).Msg("detected review request")
			} else {
				logger.Warn().Err(err).Msg("failed to encode deployment metadata")
			}
		} else {
			logger.Debug().Msg("no provider review request detected")
		}
	}()

	if commit, err := c.prj.git.wrapper.ShowCommitMetadata("HEAD"); err == nil {
		setDefaultGitMetadata(md, commit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from git")
	}

	r, err := c.prj.repo()
	if err != nil {
		printer.Stderr.WarnWithDetails("skipping fetch of review_request information", err)
		return
	}
	switch r.Host {
	case githubDomain:
		c.detectGithubMetadata(r.Owner, r.Name)
	case gitlabDomain:
		c.detectGitlabMetadata(r.Owner, r.Name)
	default:
		logger.Debug().Msgf("Skipping metadata collection for git provider: %s", r.Host)
	}
}

func (c *cli) detectGithubMetadata(owner, reponame string) {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Str("action", "detectGithubMetadata").
		Logger()

	md := c.cloud.run.metadata

	setGithubActionsMetadata(md)

	ghRepo := owner + "/" + reponame

	logger = logger.With().Str("github_repository", ghRepo).Logger()

	// HTTP Client
	githubClient := github.NewClient(&c.httpClient)

	if githubAPIURL := os.Getenv("TM_GITHUB_API_URL"); githubAPIURL != "" {
		githubBaseURL, err := url.Parse(githubAPIURL)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse github api url")
			return
		}
		githubClient.BaseURL = githubBaseURL
	}

	var prNumber int
	prFromEvent, err := tmgithub.GetEventPR()
	if err != nil {
		logger.Debug().Err(err).Msg("unable to get pull_request details from GITHUB_EVENT_PATH")
	} else {
		logger.Debug().Msg("got pull_request details from GITHUB_EVENT_PATH")
		pushedAt := prFromEvent.GetHead().GetRepo().GetPushedAt()
		c.cloud.run.rrEvent.pushedAt = pushedAt.GetTime()
		c.cloud.run.rrEvent.commitSHA = prFromEvent.GetHead().GetSHA()
		prNumber = prFromEvent.GetNumber()
	}

	ghToken, tokenSource := auth.TokenForHost(githubDomain)
	if ghToken == "" {
		printer.Stderr.WarnWithDetails(
			"Export GITHUB_TOKEN with your GitHub credentials for enabling metadata collection",
			errors.E("No GitHub token detected. Skipping the fetching of GitHub metadata."),
		)
		return
	}

	logger.Debug().Msgf("GitHub token obtained from %s", tokenSource)
	githubClient = githubClient.WithAuthToken(ghToken)

	headCommit := c.prj.headCommit()

	if ghCommit, err := getGithubCommit(githubClient, owner, reponame, headCommit); err == nil {
		setGithubCommitMetadata(md, ghCommit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from GitHub API")
	}

	pull, err := getGithubPRByNumberOrCommit(githubClient, ghToken, owner, reponame, prNumber, headCommit)
	if err != nil {
		logger.Debug().Err(err).
			Int("number", prNumber).
			Msg("failed to retrieve pull_request")
		return
	}

	logger.Debug().
		Str("pull_request_url", pull.GetHTMLURL()).
		Msg("using pull request url")

	setGithubPRMetadata(md, pull)

	reviews, err := listGithubPullReviews(githubClient, owner, reponame, pull.GetNumber())
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve PR reviews")
	}

	checks, err := listGithubChecks(githubClient, owner, reponame, headCommit)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve GHA checks")
	}

	merged := false
	if pull.GetState() == "closed" {
		merged, err = isGithubPRMerged(githubClient, owner, reponame, pull.GetNumber())
		if err != nil {
			logger.Warn().Err(err).Msg("failed to retrieve PR merged status")
		}
	}

	var reviewDecision string

	if ghToken != "" {
		httpClient := oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: ghToken},
			))

		githubQLClient := githubql.NewClient(httpClient)
		reviewDecision, err = getGithubPRReviewDecision(githubQLClient, owner, reponame, pull.GetNumber())
		if err != nil {
			logger.Warn().Err(err).Msg("failed to retrieve review decision")
		}
	}

	c.cloud.run.reviewRequest = c.newGithubReviewRequest(pull, reviews, checks, merged, reviewDecision)
}

func (c *cli) detectGitlabMetadata(group string, projectName string) {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Str("action", "detectGitlabMetadata").
		Logger()

	md := c.cloud.run.metadata
	c.setGitlabCIMetadata(md)

	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		printer.Stderr.WarnWithDetails(
			"Export GITLAB_TOKEN with your Gitlab credentials for enabling metadata collection",
			errors.E("No Gitlab token detected. Some relevant data cannot be collected."),
		)
	}

	client := gitlab.Client{
		HTTPClient: &c.httpClient,
		Group:      group,
		Project:    projectName,
		Token:      token,
	}

	if idStr := os.Getenv("CI_PROJECT_ID"); idStr != "" {
		client.ProjectID, _ = strconv.Atoi(idStr)
	}

	if gitlabAPIURL := os.Getenv("TM_GITLAB_API_URL"); gitlabAPIURL != "" {
		gitlabBaseURL, err := url.Parse(gitlabAPIURL)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse gitlab api url")
			return
		}
		client.BaseURL = gitlabBaseURL.String()
	}

	headCommit := c.prj.headCommit()
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitlabTimeout)
	defer cancel()
	mr, found, err := client.MRForCommit(ctx, headCommit)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve Merge Requests associated with commit")
		return
	}
	if !found {
		logger.Warn().Msg("No merge request associated with commit")
		return
	}
	md.GitlabMergeRequestAuthorID = mr.Author.ID
	md.GitlabMergeRequestAuthorName = mr.Author.Name
	md.GitlabMergeRequestAuthorUsername = mr.Author.Username
	md.GitlabMergeRequestAuthorAvatarURL = mr.Author.AvatarURL
	md.GitlabMergeRequestAuthorState = mr.Author.State
	md.GitlabMergeRequestAuthorWebURL = mr.Author.WebURL

	md.GitlabMergeRequestID = mr.ID
	md.GitlabMergeRequestIID = mr.IID
	md.GitlabMergeRequestState = mr.State
	md.GitlabMergeRequestCreatedAt = mr.CreatedAt
	md.GitlabMergeRequestUpdatedAt = mr.UpdatedAt
	md.GitlabMergeRequestTargetBranch = mr.TargetBranch
	md.GitlabMergeRequestSourceBranch = mr.SourceBranch
	md.GitlabMergeRequestMergeStatus = mr.MergeStatus
	md.GitlabMergeRequestWebURL = mr.WebURL

	if md.GitlabCICDBranch == "" {
		md.GitlabCICDBranch = md.GitlabMergeRequestSourceBranch
	}

	pushedAt, err := time.Parse(time.RFC3339, md.GitlabCICDJobStartedAt)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse job `started_at` field: fallback to MR `updated_at` field", err)
		pushedAt, err = time.Parse(time.RFC3339, mr.UpdatedAt)
		if err != nil {
			printer.Stderr.WarnWithDetails("failed to parse MR `updated_at` field", err)
		}
	}
	if !pushedAt.IsZero() {
		c.cloud.run.rrEvent.pushedAt = &pushedAt
	}

	c.cloud.run.rrEvent.commitSHA = mr.SHA
	c.cloud.run.reviewRequest = c.newGitlabReviewRequest(mr)
}

func (c *cli) setGitlabCIMetadata(md *cloud.DeploymentMetadata) {
	envBool := func(name string) bool {
		val := os.Getenv(name)
		return val == "true"
	}
	md.GitlabCICDJobManual = envBool("CI_JOB_MANUAL")

	// sent as string for forward-compatibility.
	md.GitlabCICDPipelineID = os.Getenv("CI_PIPELINE_ID")
	md.GitlabCICDPipelineSource = os.Getenv("CI_PIPELINE_SOURCE")
	md.GitlabCICDPipelineTriggered = envBool("CI_PIPELINE_TRIGGERED")
	md.GitlabCICDPipelineURL = os.Getenv("CI_PIPELINE_URL")
	md.GitlabCICDPipelineName = os.Getenv("CI_PIPELINE_NAME")
	md.GitlabCICDPipelineCreatedAt = os.Getenv("CI_PIPELINE_CREATED_AT")
	md.GitlabCICDJobID = os.Getenv("CI_JOB_ID")
	md.GitlabCICDJobName = os.Getenv("CI_JOB_NAME")
	md.GitlabCICDJobStartedAt = os.Getenv("CI_JOB_STARTED_AT")
	md.GitlabCICDUserEmail = os.Getenv("GITLAB_USER_EMAIL")
	md.GitlabCICDUserName = os.Getenv("GITLAB_USER_NAME")
	md.GitlabCICDUserLogin = os.Getenv("GITLAB_USER_LOGIN")
	md.GitlabCICDCommitBranch = os.Getenv("CI_COMMIT_BRANCH")
	md.GitlabCICDBranch = md.GitlabCICDCommitBranch

	createdAt, err := time.Parse(time.RFC3339, md.GitlabCICDPipelineCreatedAt)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse CI_PIPELINE_CREATED_AT time", err)
	} else {
		c.cloud.run.rrEvent.pushedAt = &createdAt
	}
	var mrApproved *bool
	if str := os.Getenv("CI_MERGE_REQUEST_APPROVED"); str != "" {
		b := str == "true"
		mrApproved = &b
	}
	md.GitlabCICDMergeRequestApproved = mrApproved
}

func getGithubPRByNumberOrCommit(githubClient *github.Client, ghToken, owner, repo string, number int, commit string) (*github.PullRequest, error) {
	logger := log.With().
		Str("github_repository", owner+"/"+repo).
		Str("commit", commit).
		Logger()

	if number != 0 {
		// fetch by number
		pull, err := getGithubPRByNumber(githubClient, owner, repo, number)
		if err != nil {
			return nil, err
		}
		return pull, nil
	}

	// fetch by commit
	pull, found, err := getGithubPRByCommit(githubClient, owner, repo, commit)
	if err != nil {
		if errors.IsKind(err, githubErrNotFound) {
			if ghToken == "" {
				logger.Warn().Msg("The GITHUB_TOKEN environment variable needs to be exported for private repositories.")
			} else {
				logger.Warn().Msg("The provided GitHub token does not have permission to read this repository or it does not exists.")
			}
		} else if errors.IsKind(err, githubErrUnprocessableEntity) {
			logger.Warn().
				Msg("The HEAD commit cannot be found in the remote. Did you forget to push?")
		} else {
			logger.Warn().
				Err(err).
				Msg("failed to retrieve pull requests associated with HEAD")
		}
		return nil, err
	}
	if !found {
		logger.Debug().Msg("no pull request associated with HEAD commit")
		return nil, stdfmt.Errorf("no pull request associated with HEAD commit")
	}

	return pull, nil
}

func getGithubCommit(ghClient *github.Client, owner, repo, commit string) (*github.RepositoryCommit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	rcommit, _, err := ghClient.Repositories.GetCommit(ctx, owner, repo, commit, nil)
	if err != nil {
		return nil, err
	}

	return rcommit, nil
}

func getGithubPRByNumber(ghClient *github.Client, owner string, repo string, number int) (*github.PullRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	pull, _, err := ghClient.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	return pull, nil

}

// returns nil, nil if there was no PR associated with commit
func getGithubPRByCommit(ghClient *github.Client, owner, repo, commit string) (*github.PullRequest, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	opt := &github.ListOptions{PerPage: 1}

	pulls, _, err := ghClient.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, commit, opt)
	if err != nil {
		return nil, true, err
	}
	if len(pulls) == 0 {
		return nil, false, nil
	}

	return pulls[0], true, nil

}

func getGithubPRReviewDecision(qlClient *githubql.Client, owner, repo string, pullNumber int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	var q struct {
		Repository struct {
			PullRequest struct {
				ReviewDecision string
			} `graphql:"pullRequest(number: $pr_number)"`
			Description string
		} `graphql:"repository(owner: $repo_owner, name: $repo_name)"`
	}

	vars := map[string]interface{}{
		"repo_owner": githubql.String(owner),
		"repo_name":  githubql.String(repo),
		"pr_number":  githubql.Int(pullNumber),
	}

	err := qlClient.Query(ctx, &q, vars)
	if err != nil {
		return "", err
	}
	r := q.Repository.PullRequest.ReviewDecision
	if r == "" {
		return "none", nil
	}
	return strings.ToLower(r), nil

}

func isGithubPRMerged(ghClient *github.Client, owner, repo string, pullNumber int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	isMerged, _, err := ghClient.PullRequests.IsMerged(ctx, owner, repo, pullNumber)
	return isMerged, err
}

func listGithubPullReviews(ghClient *github.Client, owner, repo string, pullNumber int) ([]*github.PullRequestReview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	opt := &github.ListOptions{}
	opt.PerPage = 100

	var allReviews []*github.PullRequestReview
	for {
		reviews, resp, err := ghClient.PullRequests.ListReviews(ctx, owner, repo, pullNumber, opt)
		if err != nil {
			return nil, err
		}
		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allReviews, nil
}

func listGithubChecks(ghClient *github.Client, owner, repo string, commit string) ([]*github.CheckRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	opt := &github.ListCheckRunsOptions{}
	opt.PerPage = 100

	var allChecks []*github.CheckRun
	for {
		checksResponse, resp, err := ghClient.Checks.ListCheckRunsForRef(ctx, owner, repo, commit, opt)
		if err != nil {
			return nil, err
		}
		allChecks = append(allChecks, checksResponse.CheckRuns...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allChecks, nil
}

func setDefaultGitMetadata(md *cloud.DeploymentMetadata, commit *git.CommitMetadata) {
	md.GitCommitAuthorName = commit.Author
	md.GitCommitAuthorEmail = commit.Email
	md.GitCommitAuthorTime = commit.Time
	md.GitCommitTitle = commit.Subject
	md.GitCommitDescription = commit.Body
}

func setGithubActionsMetadata(md *cloud.DeploymentMetadata) {
	md.GithubActionsDeploymentActor = os.Getenv("GITHUB_ACTOR")
	md.GithubActionsDeploymentTriggeredBy = os.Getenv("GITHUB_TRIGGERING_ACTOR")
	md.GithubActionsDeploymentBranch = os.Getenv("GITHUB_REF_NAME")
	md.GithubActionsRunID = os.Getenv("GITHUB_RUN_ID")
	md.GithubActionsRunAttempt = os.Getenv("GITHUB_RUN_ATTEMPT")
	md.GithubActionsWorkflowName = os.Getenv("GITHUB_WORKFLOW")
	md.GithubActionsWorkflowRef = os.Getenv("GITHUB_WORKFLOW_REF")
}

func setGithubCommitMetadata(md *cloud.DeploymentMetadata, commit *github.RepositoryCommit) {
	isVerified := commit.Commit.GetVerification().GetVerified()

	md.GithubCommitVerified = &isVerified
	md.GithubCommitVerifiedReason = commit.Commit.GetVerification().GetReason()

	message := commit.GetCommit().GetMessage()
	messageParts := strings.Split(message, "\n")
	md.GithubCommitTitle = messageParts[0]
	if len(messageParts) > 1 {
		md.GithubCommitDescription = strings.TrimSpace(strings.Join(messageParts[1:], "\n"))
	}

	md.GithubCommitAuthorLogin = commit.GetAuthor().GetLogin()
	md.GithubCommitAuthorAvatarURL = commit.GetAuthor().GetAvatarURL()
	md.GithubCommitAuthorGravatarID = commit.GetAuthor().GetGravatarID()

	md.GithubCommitAuthorGitName = commit.GetCommit().GetAuthor().GetName()
	md.GithubCommitAuthorGitEmail = commit.GetCommit().GetAuthor().GetEmail()
	authorDate := commit.GetCommit().GetAuthor().GetDate()
	md.GithubCommitAuthorGitDate = authorDate.GetTime()

	md.GithubCommitCommitterLogin = commit.GetCommitter().GetLogin()
	md.GithubCommitCommitterAvatarURL = commit.GetCommitter().GetAvatarURL()
	md.GithubCommitCommitterGravatarID = commit.GetCommitter().GetGravatarID()

	md.GithubCommitCommitterGitName = commit.GetCommit().GetCommitter().GetName()
	md.GithubCommitCommitterGitEmail = commit.GetCommit().GetCommitter().GetEmail()
	commiterDate := commit.GetCommit().GetCommitter().GetDate()
	md.GithubCommitCommitterGitDate = commiterDate.GetTime()
}

func setGithubPRMetadata(md *cloud.DeploymentMetadata, pull *github.PullRequest) {
	md.GithubPullRequestURL = pull.GetHTMLURL()
	md.GithubPullRequestNumber = pull.GetNumber()
	md.GithubPullRequestTitle = pull.GetTitle()
	md.GithubPullRequestDescription = pull.GetBody()
	md.GithubPullRequestState = pull.GetState()
	md.GithubPullRequestMergeCommitSHA = pull.GetMergeCommitSHA()
	md.GithubPullRequestHeadLabel = pull.GetHead().GetLabel()
	md.GithubPullRequestHeadRef = pull.GetHead().GetRef()
	md.GithubPullRequestHeadSHA = pull.GetHead().GetSHA()
	md.GithubPullRequestHeadAuthorLogin = pull.GetHead().GetUser().GetLogin()
	md.GithubPullRequestHeadAuthorAvatarURL = pull.GetHead().GetUser().GetAvatarURL()
	md.GithubPullRequestHeadAuthorGravatarID = pull.GetHead().GetUser().GetGravatarID()
	createdAt := pull.GetCreatedAt()
	updatedAt := pull.GetUpdatedAt()
	closedAt := pull.GetClosedAt()
	mergedAt := pull.GetMergedAt()
	md.GithubPullRequestCreatedAt = createdAt.GetTime()
	md.GithubPullRequestUpdatedAt = updatedAt.GetTime()
	md.GithubPullRequestClosedAt = closedAt.GetTime()
	md.GithubPullRequestMergedAt = mergedAt.GetTime()

	md.GithubPullRequestBaseLabel = pull.GetBase().GetLabel()
	md.GithubPullRequestBaseRef = pull.GetBase().GetRef()
	md.GithubPullRequestBaseSHA = pull.GetBase().GetSHA()
	md.GithubPullRequestBaseAuthorLogin = pull.GetBase().GetUser().GetLogin()
	md.GithubPullRequestBaseAuthorAvatarURL = pull.GetBase().GetUser().GetAvatarURL()
	md.GithubPullRequestBaseAuthorGravatarID = pull.GetBase().GetUser().GetGravatarID()

	md.GithubPullRequestAuthorLogin = pull.GetUser().GetLogin()
	md.GithubPullRequestAuthorAvatarURL = pull.GetUser().GetAvatarURL()
	md.GithubPullRequestAuthorGravatarID = pull.GetUser().GetGravatarID()
}

func (c *cli) newGithubReviewRequest(
	pull *github.PullRequest,
	reviews []*github.PullRequestReview,
	checks []*github.CheckRun,
	merged bool,
	reviewDecision string,
) *cloud.ReviewRequest {
	author := cloud.Author{}
	if user := pull.GetUser(); user != nil {
		author.Login = user.GetLogin()
		author.AvatarURL = user.GetAvatarURL()
	}
	pullCreatedAt := pull.GetCreatedAt()
	pullUpdatedAt := pull.GetUpdatedAt()
	rr := &cloud.ReviewRequest{
		Platform:       "github",
		Repository:     c.prj.prettyRepo(),
		URL:            pull.GetHTMLURL(),
		Number:         pull.GetNumber(),
		Title:          pull.GetTitle(),
		Description:    pull.GetBody(),
		CommitSHA:      pull.GetHead().GetSHA(),
		Draft:          pull.GetDraft(),
		ReviewDecision: reviewDecision,
		CreatedAt:      pullCreatedAt.GetTime(),
		UpdatedAt:      pullUpdatedAt.GetTime(),
		PushedAt:       c.cloud.run.rrEvent.pushedAt,
		Author:         author,
		Branch:         pull.GetHead().GetRef(),
		BaseBranch:     pull.GetBase().GetRef(),
	}

	if pull.GetState() == "closed" {
		if merged {
			rr.Status = "merged"
		} else {
			rr.Status = "closed"
		}
	} else {
		rr.Status = "open"
	}

	for _, l := range pull.Labels {
		rr.Labels = append(rr.Labels, cloud.Label{
			Name:        l.GetName(),
			Color:       l.GetColor(),
			Description: l.GetDescription(),
		})
	}

	uniqueReviewers := make(map[string]struct{})

	for _, review := range reviews {
		if review.GetState() == "CHANGES_REQUESTED" {
			rr.ChangesRequestedCount++
		} else if review.GetState() == "APPROVED" {
			rr.ApprovedCount++
		}

		login := review.GetUser().GetLogin()

		if _, found := uniqueReviewers[login]; found {
			continue
		}
		uniqueReviewers[login] = struct{}{}

		rr.Reviewers = append(rr.Reviewers, cloud.Reviewer{
			Login:     login,
			AvatarURL: review.GetUser().GetAvatarURL(),
		})
	}

	rr.ChecksTotalCount = len(checks)
	for _, check := range checks {
		switch check.GetConclusion() {
		case "success":
			rr.ChecksSuccessCount++
		case "failure":
			rr.ChecksFailureCount++
		}
	}

	return rr
}

func (c *cli) newGitlabReviewRequest(mr gitlab.MR) *cloud.ReviewRequest {
	if c.cloud.run.rrEvent.pushedAt == nil {
		panic(errors.E(errors.ErrInternal, "CI pushed_at is nil"))
	}
	mrUpdatedAt, err := time.Parse(time.RFC3339, mr.UpdatedAt)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse MR.updated_at field", err)

		t := *c.cloud.run.rrEvent.pushedAt
		mrUpdatedAt = t
	}
	var mrCreatedAt *time.Time
	if mrCreatedAtVal, err := time.Parse(time.RFC3339, mr.CreatedAt); err != nil {
		printer.Stderr.WarnWithDetails("failed to parse MR.created_at field", err)
	} else {
		mrCreatedAt = &mrCreatedAtVal
	}
	rr := &cloud.ReviewRequest{
		Platform:    "gitlab",
		Repository:  c.prj.prettyRepo(),
		URL:         mr.WebURL,
		Number:      mr.IID,
		Title:       mr.Title,
		Description: mr.Description,
		CommitSHA:   mr.SHA,
		Draft:       mr.Draft,
		CreatedAt:   mrCreatedAt,
		UpdatedAt:   &mrUpdatedAt,
		Status:      mr.State,
		Author: cloud.Author{
			Login:     mr.Author.Username,
			AvatarURL: mr.Author.AvatarURL,
		},
		Branch:     mr.SourceBranch,
		BaseBranch: mr.TargetBranch,
		// Note(i4k): PushedAt will be set only in previews.
	}

	for _, l := range mr.Labels {
		rr.Labels = append(rr.Labels, cloud.Label{
			Name: l,
		})
	}
	return rr
}

func (c *cli) loadCredential() error {
	cloudURL := cloudBaseURL()
	clientLogger := log.With().
		Str("tmc_url", cloudURL).
		Logger()

	c.cloud.client = &cloud.Client{
		BaseURL:    cloudURL,
		IDPKey:     idpkey(),
		HTTPClient: &c.httpClient,
		Logger:     &clientLogger,
	}
	c.cloud.output = c.output

	// checks if this client version can communicate with Terramate Cloud.
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	err := c.cloud.client.CheckVersion(ctx)
	if err != nil {
		return errors.E(err, clitest.ErrCloudCompat)
	}

	probes := c.credentialPrecedence(c.output)
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
		return errors.E("no credential found", ErrLoginRequired)
	}
	return nil
}

func (c *cli) ensureAllStackHaveIDs(stacks config.List[*config.SortableStack]) {
	logger := log.With().
		Str("action", "cli.ensureAllStackHaveIDs").
		Logger()

	var stacksMissingIDs []string
	for _, st := range stacks {
		if st.ID == "" {
			stacksMissingIDs = append(stacksMissingIDs, st.Dir().String())
		}
	}

	if len(stacksMissingIDs) > 0 {
		for _, stackPath := range stacksMissingIDs {
			logger.Error().Str("stack", stackPath).Msg("stack is missing the ID field")
		}
		logger.Warn().Msg("Stacks are missing IDs. You can use 'terramate create --ensure-stack-ids' to add missing IDs to all stacks.")
		c.handleCriticalError(errors.E(clitest.ErrCloudStacksWithoutID))
	}
}

func tokenClaims(token string) (jwt.MapClaims, error) {
	jwtParser := &jwt.Parser{}
	tokParsed, _, err := jwtParser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.E(err, "parsing jwt token")
	}

	if claims, ok := tokParsed.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.E("invalid jwt token claims")
}

func cloudBaseURL() string {
	var baseURL string
	cloudHost := os.Getenv("TMC_API_HOST")
	cloudURL := os.Getenv("TMC_API_URL")
	if cloudHost != "" {
		baseURL = "https://" + cloudHost
	} else if cloudURL != "" {
		baseURL = cloudURL
	} else {
		baseURL = cloud.BaseURL
	}
	return baseURL
}

func idpkey() string {
	idpKey := os.Getenv("TMC_API_IDP_KEY")
	if idpKey == "" {
		idpKey = defaultAPIKey
	}
	return idpKey
}

func cloudError() error {
	return errors.E(clitest.ErrCloud)
}

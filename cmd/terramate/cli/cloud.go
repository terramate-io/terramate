// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdjson "encoding/json"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/golang-jwt/jwt"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/printer"
	prj "github.com/terramate-io/terramate/project"
)

const (
	defaultCloudTimeout  = 60 * time.Second
	defaultGoogleTimeout = defaultCloudTimeout
	defaultGithubTimeout = defaultCloudTimeout
)

type cloudConfig struct {
	disabled bool
	client   *cloud.Client
	output   out.O

	run struct {
		runUUID cloud.UUID
		orgUUID cloud.UUID

		meta2id       map[string]int64
		reviewRequest *cloud.DeploymentReviewRequest
		metadata      *cloud.DeploymentMetadata
	}
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	IsExpired() bool
	ExpireAt() time.Time

	// private interface

	organizations() cloud.MemberOrganizations
	info()
}

type keyValue struct {
	key   string
	value string
}

func (c *cli) credentialPrecedence(output out.O) []credential {
	return []credential{
		newGithubOIDC(output, c.cloud.client),
		newGoogleCredential(output, c.cloud.client.IDPKey, c.clicfg, c.cloud.client),
	}
}

func (c *cli) cloudEnabled() bool {
	return !c.cloud.disabled
}

func (c *cli) disableCloudFeatures(err error) {
	log.Warn().Err(err).Msg(clitest.CloudDisablingMessage)

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

func (c *cli) checkCloudSync() {
	if !c.parsedArgs.Run.CloudSyncDeployment && !c.parsedArgs.Run.CloudSyncDriftStatus {
		return
	}

	err := c.setupCloudConfig()
	c.handleCriticalError(err)

	if c.cloud.disabled {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloud.run.meta2id = make(map[string]int64)
		uuid, err := generateRunID()
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

func (c *cli) setupCloudConfig() error {
	err := c.loadCredential()
	if err != nil {
		printer.Stderr.ErrorWithDetailsln("failed to load the cloud credentials", err)
		return cloudError()
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := c.cred().organizations()

	useOrgName := c.cloudOrgName()
	if useOrgName != "" {
		var useOrgUUID cloud.UUID
		for _, org := range orgs {
			if strings.EqualFold(org.Name, useOrgName) {
				if org.Status != "active" && org.Status != "trusted" {
					printer.Stderr.ErrorWithDetailsln(
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
			printer.Stderr.ErrorWithDetailsln(
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
			printer.Stderr.Errorln(clitest.CloudNoMembershipMessage)

			if len(invitedOrgs) > 0 {
				printer.Stderr.WarnWithDetailsln(
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
			printer.Stderr.ErrorWithDetailsln(
				"Invalid cloud configuration",
				errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
					"terramate.config.cloud.organization configuration attribute to a specific available organization: %s",
					activeOrgs,
				),
			)
			return cloudError()
		}

		c.cloud.run.orgUUID = activeOrgs[0].UUID
	}
	return nil
}

func (c *cli) cloudSyncBefore(run ExecContext, _ string) {
	if !c.cloudEnabled() || !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}
	c.doCloudSyncDeployment(run, deployment.Running)
}

func (c *cli) cloudSyncAfter(runContext ExecContext, res RunResult, err error) {
	if !c.cloudEnabled() || !c.isCloudSync() {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloudSyncDeployment(runContext, err)
	} else {
		c.cloudSyncDriftStatus(runContext, res, err)
	}
}

func (c *cli) cloudSyncCancelStacks(stacks []ExecContext) {
	for _, run := range stacks {
		c.cloudSyncAfter(run, RunResult{ExitCode: -1}, errors.E(ErrRunCanceled))
	}
}

func (c *cli) cloudInfo() {
	err := c.loadCredential()
	if err != nil {
		fatal(err, "failed to load credentials")
	}
	c.cred().info()
	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
}

func (c *cli) cloudDriftShow() {
	err := c.setupCloudConfig()
	if err != nil {
		fatal(err)
	}
	st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
	if err != nil {
		fatal(err, "loading stack in current directory")
	}
	if !found {
		fatal(errors.E("No stack selected. Please enter a stack to show a potential drift."))
	}
	if st.ID == "" {
		fatal(errors.E("The stack must have an ID for using TMC features"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	stackResp, found, err := c.cloud.client.GetStack(ctx, c.cloud.run.orgUUID, c.prj.prettyRepo(), st.ID)
	if err != nil {
		fatal(err)
	}
	if !found {
		fatal(errors.E("Stack %s was not yet synced with the Terramate Cloud.", st.Dir.String()))
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
		fatal(err)
	}
	if len(driftsResp.Drifts) == 0 {
		fatal(errors.E("Stack %s is drifted, but no details are available.", st.Dir.String()))
	}
	driftData := driftsResp.Drifts[0]

	ctx, cancel = context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	driftData, err = c.cloud.client.DriftDetails(ctx, c.cloud.run.orgUUID, stackResp.ID, driftData.ID)
	if err != nil {
		fatal(err)
	}
	if driftData.Status != drift.Drifted || driftData.Details == nil || driftData.Details.Provisioner == "" {
		fatal(errors.E("Stack %s is drifted, but no details are available.", st.Dir.String()))
	}
	c.output.MsgStdOutV("drift provisioner: %s", driftData.Details.Provisioner)
	c.output.MsgStdOut(driftData.Details.ChangesetASCII)
}

func (c *cli) detectCloudMetadata() {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Logger()

	prettyRepo := c.prj.prettyRepo()
	if prettyRepo == "local" {
		logger.Debug().Msg("skipping review_request and remote metadata for local repository")
		return
	}

	headCommit := c.prj.headCommit()

	c.cloud.run.metadata = &cloud.DeploymentMetadata{GitCommitSHA: headCommit}
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
	}()

	if commit, err := c.prj.git.wrapper.ShowCommitMetadata("HEAD"); err == nil {
		setDefaultGitMetadata(md, commit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from GitHub API")
	}

	r, err := repository.Parse(prettyRepo)
	if err != nil {
		logger.Debug().
			Msg("repository cannot be normalized: skipping pull request retrievals for commit")

		return
	}

	if r.Host != github.Domain {
		return
	}

	setGithubActionsMetadata(md)

	ghRepo := r.Owner + "/" + r.Name

	logger = logger.With().
		Str("github_repository", ghRepo).
		Logger()

	ghToken, tokenSource := auth.TokenForHost(r.Host)

	if ghToken != "" {
		logger.Debug().Msgf("GitHub token obtained from %s", tokenSource)
	}

	ghClient := github.Client{
		BaseURL:    os.Getenv("GITHUB_API_URL"),
		HTTPClient: &c.httpClient,
		Token:      ghToken,
	}

	if ghCommit, err := getGithubCommit(&ghClient, ghRepo, headCommit); err == nil {
		setGithubCommitMetadata(md, ghCommit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from GitHub API")
	}

	if pulls, err := getGithubPR(&ghClient, ghRepo, headCommit); err == nil {
		if len(pulls) == 0 {
			logger.Warn().
				Msg("no pull request associated with HEAD commit")

			return
		}

		for _, pull := range pulls {
			logger.Debug().
				Str("pull_request_url", pull.HTMLURL).
				Msg("found pull request")
		}

		pull := pulls[0]

		setGithubPRMetadata(md, &pull)

		logger.Debug().
			Str("pull_request_url", pull.HTMLURL).
			Msg("using pull request url")

		reviewRequest := &cloud.DeploymentReviewRequest{
			Platform:    "github",
			Repository:  c.prj.prettyRepo(),
			URL:         pull.HTMLURL,
			Number:      pull.Number,
			Title:       pull.Title,
			Description: pull.Body,
			CommitSHA:   pull.Head.SHA,
		}

		for _, l := range pull.Labels {
			reviewRequest.Labels = append(reviewRequest.Labels, cloud.Label{
				Name:        l.Name,
				Color:       l.Color,
				Description: l.Description,
			})
		}

		c.cloud.run.reviewRequest = reviewRequest

	} else {
		if errors.IsKind(err, github.ErrNotFound) {
			if ghToken == "" {
				logger.Warn().Msg("The GITHUB_TOKEN environment variable needs to be exported for private repositories.")
			} else {
				logger.Warn().Msg("The provided GitHub token does not have permission to read this repository or it does not exists.")
			}
		} else if errors.IsKind(err, github.ErrUnprocessableEntity) {
			logger.Warn().
				Msg("The HEAD commit cannot be found in the remote. Did you forget to push?")
		} else {
			logger.Warn().
				Err(err).
				Msg("failed to retrieve pull requests associated with HEAD")
		}
	}
}

func getGithubCommit(ghClient *github.Client, repo string, commitName string) (*github.Commit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	commit, err := ghClient.Commit(ctx, repo, commitName)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func getGithubPR(ghClient *github.Client, repo string, commitName string) ([]github.Pull, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	pulls, err := ghClient.PullsForCommit(ctx, repo, commitName)
	if err != nil {
		return nil, err
	}

	return pulls, nil
}

func setDefaultGitMetadata(md *cloud.DeploymentMetadata, commit *git.CommitMetadata) {
	md.GitCommitAuthorName = commit.Author
	md.GitCommitAuthorEmail = commit.Email
	md.GitCommitAuthorTime = commit.Time
	md.GitCommitTitle = commit.Subject
	md.GitCommitDescription = commit.Body
}

func setGithubActionsMetadata(md *cloud.DeploymentMetadata) {
	md.GithubActionsDeploymentTriggeredBy = os.Getenv("GITHUB_ACTOR")
	md.GithubActionsDeploymentBranch = os.Getenv("GITHUB_REF_NAME")
	md.GithubActionsRunID = os.Getenv("GITHUB_RUN_ID")
	md.GithubActionsRunAttempt = os.Getenv("GITHUB_RUN_ATTEMPT")
	md.GithubActionsWorkflowName = os.Getenv("GITHUB_WORKFLOW")
	md.GithubActionsWorkflowRef = os.Getenv("GITHUB_WORKFLOW_REF")
}

func setGithubCommitMetadata(md *cloud.DeploymentMetadata, commit *github.Commit) {
	isVerified := commit.Verification.Verified

	md.GithubCommitVerified = &isVerified
	md.GithubCommitVerifiedReason = commit.Verification.Reason

	message := commit.Commit.Message
	messageParts := strings.Split(message, "\n")
	md.GithubCommitTitle = messageParts[0]
	if len(messageParts) > 1 {
		md.GithubCommitDescription = strings.TrimSpace(strings.Join(messageParts[1:], "\n"))
	}

	md.GithubCommitAuthorLogin = commit.Author.Login
	md.GithubCommitAuthorAvatarURL = commit.Author.AvatarURL
	md.GithubCommitAuthorGravatarID = commit.Author.GravatarID

	md.GithubCommitAuthorGitName = commit.Commit.Author.Name
	md.GithubCommitAuthorGitEmail = commit.Commit.Author.Email
	md.GithubCommitAuthorGitDate = commit.Commit.Author.Date

	md.GithubCommitCommitterLogin = commit.Committer.Login
	md.GithubCommitCommitterAvatarURL = commit.Committer.AvatarURL
	md.GithubCommitCommitterGravatarID = commit.Committer.GravatarID

	md.GithubCommitCommitterGitName = commit.Commit.Committer.Name
	md.GithubCommitCommitterGitEmail = commit.Commit.Committer.Email
	md.GithubCommitCommitterGitDate = commit.Commit.Committer.Date
}

func setGithubPRMetadata(md *cloud.DeploymentMetadata, pull *github.Pull) {
	md.GithubPullRequestURL = pull.HTMLURL
	md.GithubPullRequestNumber = pull.Number
	md.GithubPullRequestTitle = pull.Title
	md.GithubPullRequestDescription = pull.Body
	md.GithubPullRequestState = pull.State
	md.GithubPullRequestMergeCommitSHA = pull.MergeCommitSHA
	md.GithubPullRequestHeadLabel = pull.Head.Label
	md.GithubPullRequestHeadRef = pull.Head.Ref
	md.GithubPullRequestHeadSHA = pull.Head.SHA
	md.GithubPullRequestHeadAuthorLogin = pull.Head.User.Login
	md.GithubPullRequestHeadAuthorAvatarURL = pull.Head.User.AvatarURL
	md.GithubPullRequestHeadAuthorGravatarID = pull.Head.User.GravatarID
	md.GithubPullRequestCreatedAt = pull.CreatedAt
	md.GithubPullRequestUpdatedAt = pull.UpdatedAt
	md.GithubPullRequestClosedAt = pull.ClosedAt
	md.GithubPullRequestMergedAt = pull.MergedAt

	md.GithubPullRequestBaseLabel = pull.Base.Label
	md.GithubPullRequestBaseRef = pull.Base.Ref
	md.GithubPullRequestBaseSHA = pull.Base.SHA
	md.GithubPullRequestBaseAuthorLogin = pull.Base.User.Login
	md.GithubPullRequestBaseAuthorAvatarURL = pull.Base.User.AvatarURL
	md.GithubPullRequestBaseAuthorGravatarID = pull.Base.User.GravatarID

	md.GithubPullRequestAuthorLogin = pull.User.Login
	md.GithubPullRequestAuthorAvatarURL = pull.User.AvatarURL
	md.GithubPullRequestAuthorGravatarID = pull.User.GravatarID
}

func (c *cli) isCloudSync() bool {
	return c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus
}

func (c *cli) loadCredential() error {
	cloudURL := cloudBaseURL()
	clientLogger := log.With().
		Str("tmc_url", cloudURL).
		Logger()

	c.cloud = cloudConfig{
		client: &cloud.Client{
			BaseURL:    cloudURL,
			IDPKey:     idpkey(),
			HTTPClient: &c.httpClient,
			Logger:     &clientLogger,
		},
		output: c.output,
	}

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
		return errors.E("no credential found")
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
		baseURL = cloudDefaultBaseURL
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

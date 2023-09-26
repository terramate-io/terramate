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
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
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
		runUUID string
		orgUUID string

		meta2id       map[string]int
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
		c.cloud.run.meta2id = make(map[string]int)
		c.cloud.run.runUUID, err = generateRunID()
		c.handleCriticalError(err)
	}
}

func (c *cli) setupCloudConfig() error {
	logger := log.With().
		Str("action", "cli.setupCloudConfig").
		Logger()

	err := c.loadCredential()
	if err != nil {
		logger.Error().Err(err).Msg("failed to load the cloud credentials")
		return cloudError()
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := c.cred().organizations()

	useOrgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if useOrgName != "" {
		var useOrgUUID string
		for _, org := range orgs {
			if org.Name == useOrgName {
				if org.Status != "active" && org.Status != "trusted" {
					logger.Error().
						Msgf("You are not yet an active member of organization %s. Please accept the invitation first.", useOrgName)

					return cloudError()
				}

				useOrgUUID = org.UUID
				break
			}
		}

		if useOrgUUID == "" {
			logger.Error().
				Msgf("You are not a member of organization %q or the organization does not exist. Available organizations: %s",
					useOrgName,
					orgs,
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
			logger.Error().Msgf(clitest.CloudNoMembershipMessage)

			if len(invitedOrgs) > 0 {
				logger.Warn().Msgf("You have pending invitation for the following organizations: %s", invitedOrgs)
			}

			return errors.E(clitest.ErrCloudOnboardingIncomplete)
		}
		if len(activeOrgs) > 1 {
			logger.Error().
				Msgf("Please set TM_CLOUD_ORGANIZATION environment variable to a specific available organization: %s", activeOrgs)

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

func (c *cli) cloudSyncAfter(runContext ExecContext, exitCode int, err error) {
	if !c.cloudEnabled() || !c.isCloudSync() {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloudSyncDeployment(runContext, err)
	} else {
		c.cloudSyncDriftStatus(runContext, exitCode, err)
	}
}

func (c *cli) cloudSyncCancelStacks(stacks []ExecContext) {
	for _, run := range stacks {
		c.cloudSyncAfter(run, -1, errors.E(ErrRunCanceled))
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

func (c *cli) detectCloudMetadata() {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Logger()

	if c.prj.prettyRepo() == "local" {
		logger.Debug().Msg("skipping review_request and metadata for local repository")
		return
	}

	r, err := repository.Parse(c.prj.prettyRepo())
	if err != nil {
		logger.Debug().
			Msg("repository cannot be normalized: skipping pull request retrievals for commit")

		return
	}

	if r.Host != github.Domain {
		return
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	headCommit := c.prj.headCommit()
	pulls, err := ghClient.PullsForCommit(ctx, ghRepo, headCommit)
	if err != nil {
		if errors.IsKind(err, github.ErrNotFound) {
			if ghToken == "" {
				logger.Warn().Msg("The GITHUB_TOKEN environment variable needs to be exported for private repositories.")
			} else {
				logger.Warn().Msg("The provided GitHub token does not have permission to read this repository or it does not exists.")
			}
			return
		}

		if errors.IsKind(err, github.ErrUnprocessableEntity) {
			logger.Warn().
				Msg("The HEAD commit cannot be found in the remote. Did you forget to push?")

			return
		}

		logger.Warn().
			Err(err).
			Msg("failed to retrieve pull requests associated with HEAD")
	}

	for _, pull := range pulls {
		logger.Debug().
			Str("pull_request_url", pull.HTMLURL).
			Msg("found pull request")
	}

	metadata := &cloud.DeploymentMetadata{
		GithubActionsDeploymentTriggeredBy: os.Getenv("GITHUB_ACTOR"),
		GithubActionsDeploymentBranch:      os.Getenv("GITHUB_REF_NAME"),
		GithubActionsRunID:                 os.Getenv("GITHUB_RUN_ID"),
		GithubActionsRunAttempt:            os.Getenv("GITHUB_RUN_ATTEMPT"),
		GitHeadCommitSHA:                   headCommit,
	}

	ctx, cancel = context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	commit, err := ghClient.Commit(ctx, ghRepo, headCommit)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from GitHub API")
	} else {
		isVerified := commit.Verification.Verified
		metadata.GithubCommitVerified = &isVerified
		metadata.GithubCommitVerifiedReason = commit.Verification.Reason

		message := commit.Commit.Message
		messageParts := strings.Split(message, "\n")
		metadata.GithubCommitTitle = messageParts[0]
		if len(messageParts) > 1 {
			metadata.GithubCommitDescription = strings.TrimSpace(strings.Join(messageParts[1:], "\n"))
		}

		metadata.GithubCommitAuthorLogin = commit.Author.Login
		metadata.GithubCommitAuthorAvatarURL = commit.Author.AvatarURL
		metadata.GithubCommitAuthorGravatarID = commit.Author.GravatarID

		metadata.GithubCommitAuthorGitName = commit.Commit.Author.Name
		metadata.GithubCommitAuthorGitEmail = commit.Commit.Author.Email
		metadata.GithubCommitAuthorGitDate = commit.Commit.Author.Date

		metadata.GithubCommitCommitterLogin = commit.Committer.Login
		metadata.GithubCommitCommitterAvatarURL = commit.Committer.AvatarURL
		metadata.GithubCommitCommitterGravatarID = commit.Committer.GravatarID

		metadata.GithubCommitCommitterGitName = commit.Commit.Committer.Name
		metadata.GithubCommitCommitterGitEmail = commit.Commit.Committer.Email
		metadata.GithubCommitCommitterGitDate = commit.Commit.Committer.Date
	}

	c.cloud.run.metadata = metadata

	if len(pulls) == 0 {
		logger.Warn().
			Msg("no pull request associated with HEAD commit")

		return
	}

	pull := pulls[0]

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

	metadata.GithubPullRequestURL = pull.HTMLURL
	metadata.GithubPullRequestNumber = pull.Number
	metadata.GithubPullRequestTitle = pull.Title
	metadata.GithubPullRequestDescription = pull.Body
	metadata.GithubPullRequestState = pull.State
	metadata.GithubPullRequestMergeCommitSHA = pull.MergeCommitSHA
	metadata.GithubPullRequestHeadLabel = pull.Head.Label
	metadata.GithubPullRequestHeadRef = pull.Head.Ref
	metadata.GithubPullRequestHeadSHA = pull.Head.SHA
	metadata.GithubPullRequestHeadAuthorLogin = pull.Head.User.Login
	metadata.GithubPullRequestHeadAuthorAvatarURL = pull.Head.User.AvatarURL
	metadata.GithubPullRequestHeadAuthorGravatarID = pull.Head.User.GravatarID
	metadata.GithubPullRequestCreatedAt = pull.CreatedAt
	metadata.GithubPullRequestUpdatedAt = pull.UpdatedAt
	metadata.GithubPullRequestClosedAt = pull.ClosedAt
	metadata.GithubPullRequestMergedAt = pull.MergedAt

	metadata.GithubPullRequestBaseLabel = pull.Base.Label
	metadata.GithubPullRequestBaseRef = pull.Base.Ref
	metadata.GithubPullRequestBaseSHA = pull.Base.SHA
	metadata.GithubPullRequestBaseAuthorLogin = pull.Base.User.Login
	metadata.GithubPullRequestBaseAuthorAvatarURL = pull.Base.User.AvatarURL
	metadata.GithubPullRequestBaseAuthorGravatarID = pull.Base.User.GravatarID

	metadata.GithubPullRequestAuthorLogin = pull.User.Login
	metadata.GithubPullRequestAuthorAvatarURL = pull.User.AvatarURL
	metadata.GithubPullRequestAuthorGravatarID = pull.User.GravatarID

	c.cloud.run.reviewRequest = reviewRequest
}

func (c *cli) isCloudSync() bool {
	return c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus
}

func (c *cli) loadCredential() error {
	c.cloud = cloudConfig{
		client: &cloud.Client{
			BaseURL:    cloudBaseURL(),
			IDPKey:     idpkey(),
			HTTPClient: &c.httpClient,
		},
		output: c.output,
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

	logger.Trace().Msg("Checking if selected stacks have id")

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

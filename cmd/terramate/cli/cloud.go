// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/golang-jwt/jwt"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

// ErrOnboardingIncomplete indicates the onboarding process is incomplete.
const ErrOnboardingIncomplete errors.Kind = "cloud commands cannot be used until onboarding is complete"

const (
	defaultCloudTimeout  = 60 * time.Second
	defaultGoogleTimeout = defaultCloudTimeout
	defaultGithubTimeout = defaultCloudTimeout
)

// DisablingCloudMessage is the message displayed in the warning when disabling
// the cloud features. It's exported because it's checked in tests.
const DisablingCloudMessage = "disabling the cloud features"

type cloudConfig struct {
	disabled bool
	client   *cloud.Client
	output   out.O

	credential credential

	run struct {
		runUUID string
		orgUUID string

		meta2id map[string]int
	}
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	IsExpired() bool
	ExpireAt() time.Time
	Validate(cloudcfg cloudConfig) error
	organizations() cloud.MemberOrganizations
	Info()
}

type keyValue struct {
	key   string
	value string
}

func (c *cli) credentialPrecedence(output out.O) []credential {
	return []credential{
		newGithubOIDC(output),
		newGoogleCredential(output, c.cloud.client.IDPKey, c.clicfg),
	}
}

func (c *cli) cloudEnabled() bool {
	return !c.cloud.disabled
}

func (c *cli) checkCloudSync() {
	if !c.parsedArgs.Run.CloudSyncDeployment && !c.parsedArgs.Run.CloudSyncDriftStatus {
		return
	}

	err := c.setupCloudConfig()
	if err != nil {
		log.Warn().Err(errors.E(err, "failed to check if credentials work")).
			Msg(DisablingCloudMessage)

		c.cloud.disabled = true
	}

	if c.cloud.disabled {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloud.run.meta2id = make(map[string]int)
		c.cloud.run.runUUID, err = generateRunID()
		if err != nil {
			fatal(err, "generating run uuid")
		}
	}
}

func (c *cli) setupCloudConfig() error {
	c.cloud = cloudConfig{
		client: &cloud.Client{
			BaseURL:    cloudBaseURL(),
			IDPKey:     idpkey(),
			HTTPClient: &c.httpClient,
		},
		output: c.output,
	}
	cred, err := c.loadCredential()
	if err != nil {
		return err
	}

	c.cloud.credential = cred
	c.cloud.client.Credential = cred
	err = cred.Validate(c.cloud)
	if err != nil {
		return err
	}

	// at this point we know user is onboarded, ie has at least 1 organization.
	orgs := c.cred().organizations()

	useOrgName := os.Getenv("TM_CLOUD_ORGANIZATION")
	if useOrgName != "" {
		var useOrgUUID string
		for _, org := range orgs {
			if org.Name == useOrgName {
				if org.Status != "active" && org.Status != "trusted" {
					fatal(errors.E("You are not yet an active member of organization %s. Please accept the invitation first.", useOrgName))
				}

				useOrgUUID = org.UUID
				break
			}
		}

		if useOrgUUID == "" {
			fatal(errors.E("You are not a member of organization %q or the organization does not exist. Available organizations: %s",
				useOrgName,
				orgs,
			))
		}

		c.cloud.run.orgUUID = useOrgUUID
	} else if len(orgs) != 1 {
		fatal(
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable to a specific available organization: %s", orgs),
		)
	} else {
		org := orgs[0]
		if org.Status != "active" && org.Status != "trusted" {
			fatal(errors.E("You are not yet an active member of organization %s. Please accept the invitation first.", org.Name))
		}
		c.cloud.run.orgUUID = org.UUID
	}

	return nil
}

func (c *cli) cloudSyncBefore(s *config.Stack, _ string) {
	if !c.cloudEnabled() || !c.parsedArgs.Run.CloudSyncDeployment {
		return
	}
	c.doCloudSyncDeployment(s, deployment.Running)
}

func (c *cli) cloudSyncAfter(s *config.Stack, exitCode int, err error) {
	if !c.cloudEnabled() || !c.isCloudSync() {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloudSyncDeployment(s, err)
	} else {
		c.cloudSyncDriftStatus(s, exitCode, err)
	}
}

func (c *cli) cloudSyncCancelStacks(stacks []ExecContext) {
	for _, run := range stacks {
		c.cloudSyncAfter(run.Stack, -1, errors.E(ErrRunCanceled))
	}
}

func (c *cli) cloudInfo() {
	err := c.setupCloudConfig()
	if err != nil {
		fatal(err)
	}
	c.cred().Info()
	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
}

func (c *cli) tryGithubMetadata() (*cloud.DeploymentReviewRequest, *cloud.DeploymentMetadata, string) {
	logger := log.With().
		Str("normalized_repository", c.prj.prettyRepo()).
		Str("head_commit", c.prj.headCommit()).
		Logger()

	r, err := repository.Parse(c.prj.prettyRepo())
	if err != nil {
		logger.Debug().
			Msg("repository cannot be normalized: skipping pull request retrievals for commit")

		return nil, nil, ""
	}

	if r.Host != github.Domain {
		return nil, nil, ""
	}

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
			return nil, nil, ghRepo
		}

		if errors.IsKind(err, github.ErrUnprocessableEntity) {
			logger.Warn().
				Msg("The HEAD commit cannot be found in the remote. Did you forget to push?")

			return nil, nil, ghRepo
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
		Platform:              "github",
		DeploymentTriggeredBy: os.Getenv("GITHUB_ACTOR"),
		DeploymentBranch:      os.Getenv("GITHUB_REF_NAME"),
		DeploymentCommitSHA:   headCommit,
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
		metadata.DeploymentCommitVerified = &isVerified
		metadata.DeploymentCommitVerifiedReason = commit.Verification.Reason

		message := commit.Commit.Message
		messageParts := strings.Split(message, "\n")
		metadata.DeploymentCommitTitle = messageParts[0]
		if len(messageParts) > 1 {
			metadata.DeploymentCommitDescription = strings.Join(messageParts[1:], "\n")
		}

		metadata.DeploymentCommitAuthorLogin = commit.Author.Login
		metadata.DeploymentCommitAuthorAvatarURL = commit.Author.AvatarURL
		metadata.DeploymentCommitAuthorGravatarID = commit.Author.GravatarID

		metadata.DeploymentCommitAuthorGitName = commit.Commit.Author.Name
		metadata.DeploymentCommitAuthorGitEmail = commit.Commit.Author.Email
		metadata.DeploymentCommitAuthorGitDate = commit.Commit.Author.Date

		metadata.DeploymentCommitCommitterLogin = commit.Committer.Login
		metadata.DeploymentCommitCommitterAvatarURL = commit.Committer.AvatarURL
		metadata.DeploymentCommitCommitterGravatarID = commit.Committer.GravatarID

		metadata.DeploymentCommitCommitterGitName = commit.Commit.Committer.Name
		metadata.DeploymentCommitCommitterGitEmail = commit.Commit.Committer.Email
		metadata.DeploymentCommitCommitterGitDate = commit.Commit.Committer.Date
	}

	if len(pulls) == 0 {
		logger.Warn().
			Msg("no pull request associated with HEAD commit")

		return nil, metadata, ghRepo
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

	metadata.PullRequestAuthorLogin = pull.User.Login
	metadata.PullRequestAuthorAvatarURL = pull.User.AvatarURL
	metadata.PullRequestAuthorGravatarID = pull.User.GravatarID
	metadata.PullRequestHeadLabel = pull.Head.Label
	metadata.PullRequestHeadRef = pull.Head.Ref
	metadata.PullRequestHeadSHA = pull.Head.SHA
	metadata.PullRequestHeadAuthorLogin = pull.Head.User.Login
	metadata.PullRequestHeadAuthorAvatarURL = pull.Head.User.AvatarURL
	metadata.PullRequestHeadAuthorGravatarID = pull.Head.User.GravatarID

	metadata.PullRequestBaseLabel = pull.Base.Label
	metadata.PullRequestBaseRef = pull.Base.Ref
	metadata.PullRequestBaseSHA = pull.Base.SHA
	metadata.PullRequestBaseAuthorLogin = pull.Base.User.Login
	metadata.PullRequestBaseAuthorAvatarURL = pull.Base.User.AvatarURL
	metadata.PullRequestBaseAuthorGravatarID = pull.Base.User.GravatarID

	metadata.PullRequestCreatedAt = pull.CreatedAt
	metadata.PullRequestUpdatedAt = pull.UpdatedAt
	metadata.PullRequestClosedAt = pull.ClosedAt
	metadata.PullRequestMergedAt = pull.MergedAt
	return reviewRequest, metadata, ghToken
}

func (c *cli) isCloudSync() bool {
	return c.parsedArgs.Run.CloudSyncDeployment || c.parsedArgs.Run.CloudSyncDriftStatus
}

func (c *cli) loadCredential() (credential, error) {
	probes := c.credentialPrecedence(c.output)
	var cred credential
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return nil, err
		}
		if found {
			cred = probe
			break
		}
	}
	if !found {
		return nil, errors.E("no credential found")
	}

	return cred, nil
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
	if cloudHost != "" {
		baseURL = "https://" + cloudHost
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

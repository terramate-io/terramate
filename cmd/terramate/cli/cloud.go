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
	"golang.org/x/oauth2"

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

const githubDomain = "github.com"

const (
	githubErrNotFound            errors.Kind = "resource not found (HTTP Status: 404)"
	githubErrUnprocessableEntity errors.Kind = "entity cannot be processed (HTTP Status: 422)"
)

type cloudConfig struct {
	disabled bool
	client   *cloud.Client
	output   out.O

	run struct {
		runUUID cloud.UUID
		orgName string
		orgUUID cloud.UUID

		meta2id map[string]int64
		// stackPreviews is a map of stack.ID to stackPreview.ID
		stackPreviews               map[string]string
		reviewRequest               *cloud.ReviewRequest
		prFromGHAEvent              *github.PullRequest
		metadata                    *cloud.DeploymentMetadata
		technology, technologyLayer string
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
	info(selectedOrgName string)
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
			fatal("aborting", err)
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
	if !c.parsedArgs.Run.CloudSyncDeployment && !c.parsedArgs.Run.CloudSyncDriftStatus && !c.parsedArgs.Run.CloudSyncPreview {
		return
	}

	err := c.setupCloudConfig()
	c.handleCriticalError(err)

	if c.cloud.disabled {
		return
	}

	if c.parsedArgs.Run.CloudSyncDeployment {
		c.cloud.run.meta2id = make(map[string]int64)
		uuid, err := uuid.GenerateUUID()
		c.handleCriticalError(err)
		c.cloud.run.runUUID = cloud.UUID(uuid)
	}

	if c.parsedArgs.Run.CloudSyncPreview {
		c.cloud.run.stackPreviews = make(map[string]string)
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
	stackPreviewID := c.cloud.run.stackPreviews[run.Stack.ID]
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
	planfile := c.parsedArgs.Run.CloudSyncTerraformPlanFile

	previewStatus := preview.DerivePreviewStatus(res.ExitCode)
	var previewChangeset *cloud.ChangesetDetails
	if planfile != "" {
		changeset, err := c.getTerraformChangeset(run, planfile)
		if err != nil || changeset == nil {
			printer.Stderr.WarnWithDetails(
				sprintf("skipping terraform plan sync for %s", run.Stack.Dir.String()),
				err)

			printer.Stderr.Warn(
				sprintf("preview status set to \"failed\" (previously %q) due to failure when generating the "+
					"changeset details", previewStatus),
			)

			previewStatus = preview.StackStatusFailed
		}
		if changeset != nil {
			previewChangeset = &cloud.ChangesetDetails{
				Provisioner:    changeset.Provisioner,
				ChangesetASCII: changeset.ChangesetASCII,
				ChangesetJSON:  changeset.ChangesetJSON,
			}
		}
	}

	stackPreviewID := c.cloud.run.stackPreviews[run.Stack.ID]
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
		fatal("failed to load credentials", err)
	}
	c.cred().info(c.cloudOrgName())

	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
}

func (c *cli) cloudDriftShow() {
	err := c.setupCloudConfig()
	if err != nil {
		fatal("unable to setup cloud configuration", err)
	}
	st, found, err := config.TryLoadStack(c.cfg(), prj.PrjAbsPath(c.rootdir(), c.wd()))
	if err != nil {
		fatal("loading stack in current directory", err)
	}
	if !found {
		fatal("No stack selected. Please enter a stack to show a potential drift.", nil)
	}
	if st.ID == "" {
		fatal("The stack must have an ID for using TMC features", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	stackResp, found, err := c.cloud.client.GetStack(ctx, c.cloud.run.orgUUID, c.prj.prettyRepo(), st.ID)
	if err != nil {
		fatal("unable to fetch stack", err)
	}
	if !found {
		fatal(sprintf("Stack %s was not yet synced with the Terramate Cloud.", st.Dir.String()), nil)
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
		fatal("unable to fetch drift", err)
	}
	if len(driftsResp.Drifts) == 0 {
		fatal(sprintf("Stack %s is drifted, but no details are available.", st.Dir.String()), nil)
	}
	driftData := driftsResp.Drifts[0]

	ctx, cancel = context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	driftData, err = c.cloud.client.DriftDetails(ctx, c.cloud.run.orgUUID, stackResp.ID, driftData.ID)
	if err != nil {
		fatal("unable to fetch drift details", err)
	}
	if driftData.Status != drift.Drifted || driftData.Details == nil || driftData.Details.Provisioner == "" {
		fatal(sprintf("Stack %s is drifted, but no details are available.", st.Dir.String()), nil)
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

	c.cloud.run.metadata = &cloud.DeploymentMetadata{}
	c.cloud.run.metadata.GitCommitSHA = headCommit

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

	if r.Host != githubDomain {
		return
	}

	setGithubActionsMetadata(md)

	ghRepo := r.Owner + "/" + r.Name

	logger = logger.With().
		Str("github_repository", ghRepo).
		Logger()

	// HTTP Client
	githubClient := github.NewClient(&c.httpClient)

	ghToken, tokenSource := auth.TokenForHost(r.Host)

	if ghToken != "" {
		logger.Debug().Msgf("GitHub token obtained from %s", tokenSource)
		githubClient = githubClient.WithAuthToken(ghToken)
	}

	// GraphQL CLient
	var githubQLClient *githubql.Client
	if ghToken != "" {
		httpClient := oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: ghToken},
			))

		githubQLClient = githubql.NewClient(httpClient)
	} else {
		githubQLClient = githubql.NewClient(&c.httpClient)
	}

	if ghCommit, err := getGithubCommit(githubClient, r.Owner, r.Name, headCommit); err == nil {
		setGithubCommitMetadata(md, ghCommit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from GitHub API")
	}

	var prNumber int
	prFromEvent, err := tmgithub.GetEventPR()
	if err != nil {
		logger.Debug().Err(err).Msg("unable to get pull_request details from GITHUB_EVENT_PATH")
	} else {
		logger.Debug().Err(err).Msg("got pull_request details from GITHUB_EVENT_PATH")
		c.cloud.run.prFromGHAEvent = prFromEvent
		prNumber = prFromEvent.GetNumber()
	}

	pull, err := getGithubPRByNumberOrCommit(githubClient, ghToken, r.Owner, r.Name, prNumber, headCommit)
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

	reviews, err := listGithubPullReviews(githubClient, r.Owner, r.Name, pull.GetNumber())
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve PR reviews")
	}

	checks, err := listGithubChecks(githubClient, r.Owner, r.Name, headCommit)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve PR reviews")
	}

	merged := false
	if pull.GetState() == "closed" {
		merged, err = isGithubPRMerged(githubClient, r.Owner, r.Name, pull.GetNumber())
		if err != nil {
			logger.Warn().Err(err).Msg("failed to retrieve PR merged status")
		}
	}

	reviewDecision, err := getGithubPRReviewDecision(githubQLClient, r.Owner, r.Name, pull.GetNumber())
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve review decision")
	}

	c.cloud.run.reviewRequest = c.newReviewRequest(pull, reviews, checks, merged, reviewDecision)
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
		logger.Warn().Msg("no pull request associated with HEAD commit")
		return nil, err
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

	opt := &github.ListOptions{PerPage: 100}

	var allReviews []*github.PullRequestReview
	for {
		reviews, resp, err := ghClient.PullRequests.ListReviews(ctx, owner, repo, pullNumber, nil)
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

	opt := &github.ListOptions{PerPage: 100}

	var allChecks []*github.CheckRun
	for {
		checksResponse, resp, err := ghClient.Checks.ListCheckRunsForRef(ctx, owner, repo, commit, nil)
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
	md.GithubActionsDeploymentTriggeredBy = os.Getenv("GITHUB_ACTOR")
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
	md.GithubPullRequestCreatedAt = createdAt.GetTime()
	md.GithubPullRequestUpdatedAt = updatedAt.GetTime()
	md.GithubPullRequestClosedAt = pull.ClosedAt.GetTime()
	md.GithubPullRequestMergedAt = pull.MergedAt.GetTime()

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

func (c *cli) newReviewRequest(pull *github.PullRequest, reviews []*github.PullRequestReview, checks []*github.CheckRun, merged bool, reviewDecision string) *cloud.ReviewRequest {
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
		UpdatedAt:      pullUpdatedAt.GetTime(),
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

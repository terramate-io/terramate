// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"
	gh "github.com/google/go-github/v58/github"
	githubql "github.com/shurcooL/githubv4"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/ci"
	"github.com/terramate-io/terramate/cloud/api/metadata"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/integrations/bitbucket"
	"github.com/terramate-io/terramate/cloud/integrations/github"
	"github.com/terramate-io/terramate/cloud/integrations/gitlab"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/strconv"
	"golang.org/x/oauth2"
)

const githubDomain = "github.com"
const gitlabDomain = "gitlab.com"
const bitbucketDomain = "bitbucket.org"

const (
	errGithubNotFound            errors.Kind = "resource not found (HTTP Status: 404)"
	errGithubUnprocessableEntity errors.Kind = "entity cannot be processed (HTTP Status: 422)"
)

// DetectCloudMetadata detects the cloud metadata for the current run.
func DetectCloudMetadata(e *engine.Engine, state *CloudRunState) {
	prj := e.Project()
	repo, err := prj.Repo()
	if err != nil {
		e.DisableCloudFeatures(err)
		return
	}
	if repo.Repo == "" {
		e.DisableCloudFeatures(errors.E("failed to canonicalize the repository URL"))
		return
	}
	logger := log.With().
		Str("normalized_repository", repo.Repo).
		Str("action", "detectCloudMetadata").
		Logger()

	if repo.Repo == "local" {
		e.DisableCloudFeatures(errors.E("skipping review_request and remote metadata for local repository"))
		return
	}

	state.Metadata = &resources.DeploymentMetadata{}
	state.Metadata.GitCommitSHA, err = prj.HeadCommit()
	if err != nil {
		e.DisableCloudFeatures(err)
		return
	}

	md := state.Metadata

	defer func() {
		if state.Metadata != nil {
			data, err := json.Marshal(state.Metadata)
			if err == nil {
				logger.Debug().RawJSON("provider_metadata", data).Msg("detected metadata")
			} else {
				logger.Warn().Err(err).Msg("failed to encode deployment metadata")
			}
		} else {
			logger.Debug().Msg("no provider metadata detected")
		}

		if state.ReviewRequest != nil {
			data, err := json.Marshal(state.ReviewRequest)
			if err == nil {
				logger.Debug().RawJSON("provider_review_request", data).Msg("detected review request")
			} else {
				logger.Warn().Err(err).Msg("failed to encode deployment metadata")
			}
		} else {
			logger.Debug().Msg("no provider review request detected")
		}
	}()

	if commit, err := prj.Git.Wrapper.ShowCommitMetadata("HEAD"); err == nil {
		setDefaultGitMetadata(md, commit)
	} else {
		logger.Warn().
			Err(err).
			Msg("failed to retrieve commit information from git")
	}

	switch prj.CIPlatform() {
	case ci.PlatformGithub:
		detectGithubMetadata(e, repo.Owner, repo.Name, state)
	case ci.PlatformGitlab:
		detectGitlabMetadata(e, repo.Owner, repo.Name, state)
	case ci.PlatformBitBucket:
		detectBitbucketMetadata(e, repo.Owner, repo.Name, state)
	case ci.PlatformLocal:
		// in case of running locally, we collect the metadata based on the repository host.
		switch repo.Host {
		case githubDomain:
			detectGithubMetadata(e, repo.Owner, repo.Name, state)
		case gitlabDomain:
			detectGitlabMetadata(e, repo.Owner, repo.Name, state)
		case bitbucketDomain:
			detectBitbucketMetadata(e, repo.Owner, repo.Name, state)
		}
	default:
		logger.Debug().Msgf("Skipping metadata collection for ci provider: %s", prj.CIPlatform())
	}
}

func detectGithubMetadata(e *engine.Engine, owner, reponame string, state *CloudRunState) {
	prj := e.Project()

	prettyRepo, _ := prj.PrettyRepo()
	logger := log.With().
		Str("normalized_repository", prettyRepo).
		Str("action", "detectGithubMetadata").
		Logger()

	md := state.Metadata

	setGithubActionsMetadata(md)

	ghRepo := owner + "/" + reponame

	logger = logger.With().Str("github_repository", ghRepo).Logger()

	// HTTP Client
	githubClient := gh.NewClient(&e.HTTPClient)

	if githubAPIURL := os.Getenv("TM_GITHUB_API_URL"); githubAPIURL != "" {
		githubBaseURL, err := url.Parse(githubAPIURL)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse github api url")
			return
		}
		githubClient.BaseURL = githubBaseURL
	}

	var prNumber int
	prFromEvent, err := github.GetEventPR()
	if err != nil {
		logger.Debug().Err(err).Msg("unable to get pull_request details from GITHUB_EVENT_PATH")
	} else {
		logger.Debug().Msg("got pull_request details from GITHUB_EVENT_PATH")
		pushedAt := prFromEvent.GetHead().GetRepo().GetPushedAt()
		var pushedInt int64
		if t := pushedAt.GetTime(); t != nil {
			pushedInt = t.Unix()
		}
		state.RREvent.PushedAt = &pushedInt
		state.RREvent.CommitSHA = prFromEvent.GetHead().GetSHA()
		prNumber = prFromEvent.GetNumber()
	}

	ghToken, tokenSource := ghauth.TokenForHost(githubDomain)
	if ghToken == "" {
		printer.Stderr.WarnWithDetails(
			"Export GITHUB_TOKEN with your GitHub credentials for enabling metadata collection",
			errors.E("No GitHub token detected. Skipping the fetching of GitHub metadata."),
		)
		return
	}

	logger.Debug().Msgf("GitHub token obtained from %s", tokenSource)
	githubClient = githubClient.WithAuthToken(ghToken)

	headCommit, err := prj.HeadCommit()
	if err != nil {
		e.DisableCloudFeatures(err)
		return
	}

	logger = logger.With().Str("head_commit", headCommit).Logger()

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

	state.ReviewRequest = newGithubReviewRequest(e, pull, reviews, checks, merged, reviewDecision, state)

	// New grouping structure.
	md.GithubPullRequest = metadata.NewGithubPullRequest(pull, reviews)
}

func detectGitlabMetadata(e *engine.Engine, group string, projectName string, state *CloudRunState) {
	prj := e.Project()
	prettyRepo, _ := prj.PrettyRepo()
	logger := log.With().
		Str("normalized_repository", prettyRepo).
		Str("action", "detectGitlabMetadata").
		Logger()

	md := state.Metadata
	setGitlabCIMetadata(e, md, state)

	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		printer.Stderr.WarnWithDetails(
			"Export GITLAB_TOKEN with your Gitlab credentials for enabling metadata collection",
			errors.E("No Gitlab token detected. Some relevant data cannot be collected."),
		)
	}

	client := gitlab.Client{
		HTTPClient: &e.HTTPClient,
		Group:      group,
		Project:    projectName,
		Token:      token,
	}

	if idStr := os.Getenv("CI_PROJECT_ID"); idStr != "" {
		client.ProjectID, _ = strconv.Atoi64(idStr)
	}

	if gitlabAPIURL := os.Getenv("TM_GITLAB_API_URL"); gitlabAPIURL != "" {
		gitlabBaseURL, err := url.Parse(gitlabAPIURL)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse gitlab api url")
			return
		}
		client.BaseURL = gitlabBaseURL.String()
	}

	headCommit, err := prj.HeadCommit()
	if err != nil {
		e.DisableCloudFeatures(err)
		return
	}

	logger = logger.With().Str("head_commit", headCommit).Logger()

	ctx, cancel := context.WithTimeout(context.Background(), gitlab.DefaultTimeout)
	defer cancel()

	// In merged results pipelines, headCommit will point to the temporary local commit that
	// Gitlab creates when executing the pipeline, and we won't be able to use that to locate
	// the relevant merge request. Instead, we use the CI_MERGE_REQUEST_SOURCE_BRANCH_SHA variable
	// to request MR details when it is available.
	mrCommitSha := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
	if mrCommitSha == "" {
		mrCommitSha = headCommit
	}

	mr, found, err := client.MRForCommit(ctx, mrCommitSha)
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
		pushedAtInt := pushedAt.Unix()
		state.RREvent.PushedAt = &pushedAtInt
	}

	state.RREvent.CommitSHA = mr.SHA
	state.ReviewRequest = newGitlabReviewRequest(e, mr, state)

	reviewers, err := client.MRReviewers(ctx, mr.IID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve Merge Request reviewers")
		return
	}

	participants, err := client.MRParticipants(ctx, mr.IID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to retrieve Merge Request participants")
		return
	}

	// New grouping structure.
	md.GitlabMergeRequest = metadata.NewGitlabMergeRequest(&mr, reviewers, participants)
}

func detectBitbucketMetadata(e *engine.Engine, owner, reponame string, state *CloudRunState) {
	prj := e.Project()
	prettyRepo, _ := prj.PrettyRepo()
	logger := log.With().
		Str("normalized_repository", prettyRepo).
		Str("action", "detectBitbucketMetadata").
		Logger()

	md := state.Metadata

	setBitbucketPipelinesMetadata(e, md)

	if md.BitbucketPipelinesBuildNumber == "" {
		printer.Stderr.Warn("No Bitbucket CI build number detected. Skipping metadata collection.")
		return
	}

	token := os.Getenv("BITBUCKET_TOKEN")

	if token == "" {
		printer.Stderr.WarnWithDetails(
			"Export BITBUCKET_TOKEN with your Bitbucket access token for enabling metadata collection",
			errors.E("No Bitbucket token detected. Some relevant data cannot be collected."),
		)
	}

	client := bitbucket.Client{
		HTTPClient: &e.HTTPClient,
		Workspace:  owner,
		RepoSlug:   reponame,
		Token:      token,
	}

	ctx, cancel := context.WithTimeout(context.Background(), bitbucket.DefaultTimeout)
	defer cancel()

	// Try to get PR directly if ID is available (more reliable)
	prIDStr := os.Getenv("BITBUCKET_PR_ID")
	var pullRequest *bitbucket.PR

	if prIDStr != "" {
		id64, err := strconv.Atoi64(prIDStr)
		if err != nil {
			printer.Stderr.WarnWithDetails(fmt.Sprintf("Failed to parse BITBUCKET_PR_ID %q as int", prIDStr), err)
		} else {
			id := int(id64)
			logger.Debug().
				Int("pr_id", id).
				Msg("fetching pull request by id")

			pr, err := client.GetPullRequest(ctx, id)
			if err != nil {
				printer.Stderr.WarnWithDetails("failed to retrieve pull request by ID", err)
			} else {
				// Verify this PR matches the commit if we have one, strictly speaking not required but good for safety
				// In pipelines, PR_ID implies context is that PR.
				pullRequest = &pr
			}
		}
	}

	// Fallback to commit search if PR not found by ID
	var prs []bitbucket.PR
	if pullRequest == nil {

		var err error
		prs, err = client.GetPullRequestsForCommit(ctx, md.BitbucketPipelinesCommit, md.BitbucketPipelinesBranch)
		if err != nil {
			printer.Stderr.WarnWithDetails(
				"failed to retrieve pull requests associated with commit. "+
					"Check if the Bitbucket token is valid and has the required permissions. "+
					"Note that Bitbucket requires enabling the Pull Requests API in the UI. "+
					"Check our Bitbucket documentation page at https://terramate.io/docs/cli/automation/bitbucket-pipelines/",
				err,
			)
			return
		}

		pullRequest = findMatchingBitbucketPR(prs, md.BitbucketPipelinesCommit, md.BitbucketPipelinesBranch, md.BitbucketPipelinesDestinationBranch)
	}

	if pullRequest == nil {
		if len(prs) == 0 {
			printer.Stderr.Warn(fmt.Sprintf("No pull request associated with commit %q", md.BitbucketPipelinesCommit))
		} else {
			printer.Stderr.Warn("No pull request found with matching source and destination branches")
		}
		return
	}

	// The pullRequest.source.commit.hash is a short 12 character commit hash

	var commitHash string
	commit, err := client.GetCommit(ctx, pullRequest.Source.Commit.ShortHash)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to retrieve commit information", err)
	} else {
		commitHash = commit.ShortHash
	}

	pullRequest.Source.Commit.SHA = commitHash

	buildNumber, err := strconv.Atoi64(md.BitbucketPipelinesBuildNumber)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse Bitbucket CI build number", err)
		return
	}

	state.RREvent.PushedAt = &buildNumber
	state.RREvent.CommitSHA = commitHash
	state.ReviewRequest = newBitbucketReviewRequest(e, pullRequest)

	// New grouping structure.
	md.BitbucketPullRequest = metadata.NewBitbucketPullRequest(pullRequest)

	logger.Debug().Msg("Bitbucket metadata detected")
}

func setGitlabCIMetadata(_ *engine.Engine, md *resources.DeploymentMetadata, state *CloudRunState) {
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
	md.GitlabCICDUserID = os.Getenv("GITLAB_USER_ID")
	md.GitlabCICDUserLogin = os.Getenv("GITLAB_USER_LOGIN")
	md.GitlabCICDCommitBranch = os.Getenv("CI_COMMIT_BRANCH")
	md.GitlabCICDBranch = md.GitlabCICDCommitBranch
	md.GitlabCIServerHost = os.Getenv("CI_SERVER_HOST")
	md.GitlabCIServerURL = os.Getenv("CI_SERVER_URL")

	createdAt, err := time.Parse(time.RFC3339, md.GitlabCICDPipelineCreatedAt)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse CI_PIPELINE_CREATED_AT time", err)
	} else {
		createdAtInt := createdAt.Unix()
		state.RREvent.PushedAt = &createdAtInt
	}
	var mrApproved *bool
	if str := os.Getenv("CI_MERGE_REQUEST_APPROVED"); str != "" {
		b := str == "true"
		mrApproved = &b
	}
	md.GitlabCICDMergeRequestApproved = mrApproved
}

func setBitbucketPipelinesMetadata(e *engine.Engine, md *resources.DeploymentMetadata) {
	md.BitbucketPipelinesBuildNumber = os.Getenv("BITBUCKET_BUILD_NUMBER")
	md.BitbucketPipelinesPipelineUUID = os.Getenv("BITBUCKET_PIPELINE_UUID")
	md.BitbucketPipelinesCommit = os.Getenv("BITBUCKET_COMMIT")
	md.BitbucketPipelinesWorkspace = os.Getenv("BITBUCKET_WORKSPACE")
	md.BitbucketPipelinesRepoSlug = os.Getenv("BITBUCKET_REPO_SLUG")
	md.BitbucketPipelinesRepoUUID = os.Getenv("BITBUCKET_REPO_UUID")
	md.BitbucketPipelinesRepoFullName = os.Getenv("BITBUCKET_REPO_FULL_NAME")
	md.BitbucketPipelinesBranch = os.Getenv("BITBUCKET_BRANCH")
	md.BitbucketPipelinesTag = os.Getenv("BITBUCKET_TAG")
	md.BitbucketPipelinesParallelStep = os.Getenv("BITBUCKET_PARALLEL_STEP")
	md.BitbucketPipelinesParallelStepCount = os.Getenv("BITBUCKET_PARALLEL_STEP_COUNT")
	md.BitbucketPipelinesPRID = os.Getenv("BITBUCKET_PR_ID")
	md.BitbucketPipelinesDestinationBranch = os.Getenv("BITBUCKET_PR_DESTINATION_BRANCH")
	md.BitbucketPipelinesStepUUID = os.Getenv("BITBUCKET_STEP_UUID")
	md.BitbucketPipelinesDeploymentEnvironment = os.Getenv("BITBUCKET_DEPLOYMENT_ENVIRONMENT")
	md.BitbucketPipelinesDeploymentEnvironmentUUID = os.Getenv("BITBUCKET_DEPLOYMENT_ENVIRONMENT_UUID")
	md.BitbucketPipelinesProjectKey = os.Getenv("BITBUCKET_PROJECT_KEY")
	md.BitbucketPipelinesProjectUUID = os.Getenv("BITBUCKET_PROJECT_UUID")
	md.BitbucketPipelinesStepTriggererUUID = os.Getenv("BITBUCKET_STEP_TRIGGERER_UUID")

	client := bitbucket.Client{
		HTTPClient: &e.HTTPClient,
	}

	ctx, cancel := context.WithTimeout(context.Background(), bitbucket.DefaultTimeout)
	defer cancel()
	user, err := client.GetUser(ctx, md.BitbucketPipelinesStepTriggererUUID)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to retrieve user information", err)
	} else {
		md.BitbucketPipelinesTriggeredByAccountID = user.AccountID
		md.BitbucketPipelinesTriggeredByNickname = user.Nickname
		md.BitbucketPipelinesTriggeredByDisplayName = user.DisplayName
		md.BitbucketPipelinesTriggeredByAvatarURL = user.Links.Avatar.Href
	}
}

func findMatchingBitbucketPR(prs []bitbucket.PR, commit, branch, destBranch string) *bitbucket.PR {
	match := func(sha1, sha2 string) bool {
		if sha1 == "" || sha2 == "" {
			return false
		}
		return strings.HasPrefix(sha1, sha2) || strings.HasPrefix(sha2, sha1)
	}

	for _, pr := range prs {
		pr := pr // fix loop variable capturing

		// only PR events have source and destination branches
		if destBranch != "" &&
			pr.Source.Branch.Name == branch &&
			pr.Destination.Branch.Name == destBranch {
			return &pr
		}

		if match(commit, pr.MergeCommit.ShortHash) || match(commit, pr.Source.Commit.ShortHash) {
			// the pr.MergeCommit.Hash and pr.Source.Commit.Hash contains a short 12 character commit hash
			return &pr
		}
	}
	return nil
}

func newBitbucketReviewRequest(e *engine.Engine, pr *bitbucket.PR) *resources.ReviewRequest {
	createdAt, err := time.Parse(time.RFC3339, pr.CreatedOn)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse PR created_on time", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, pr.UpdatedOn)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse PR updated_on time", err)
	}

	uniqueReviewers := make(map[string]resources.Reviewer)

	var reviewerApprovalCount int
	var reviewerChangesRequestedCount int
	var changesRequestedCount int
	var approvalCount int
	var reviewers []resources.Reviewer
	for _, participant := range pr.Participants {
		state, ok := participant.State.(string)
		if !ok {
			continue
		}

		if participant.Role == "REVIEWER" {
			uniqueReviewers[participant.User.DisplayName] = resources.Reviewer{
				Login:     participant.User.DisplayName,
				AvatarURL: participant.User.Links.Avatar.Href,
				ID:        participant.User.UUID,
			}
		}

		switch state {
		case "changes_requested":
			changesRequestedCount++
			if participant.Role == "REVIEWER" {
				reviewerChangesRequestedCount++
			}
		case "approved":
			approvalCount++
			if participant.Role == "REVIEWER" {
				reviewerApprovalCount++
			}
		}

	}
	for _, reviewer := range uniqueReviewers {
		reviewers = append(reviewers, reviewer)
	}

	// TODO(i4k): Bitbucket does not provide a final review decision from the API but we
	// can infer it from the reviewers + participants fields.
	reviewDecision := ""
	if len(pr.Reviewers) > 0 {
		reviewDecision = "review_required"
		if reviewerApprovalCount > 0 {
			reviewDecision = "approved"
		}
		if reviewerChangesRequestedCount > 0 {
			reviewDecision = "changes_requested"
		}
	}

	repo, err := e.Project().PrettyRepo()
	if err != nil {
		e.DisableCloudFeatures(err)
		return nil
	}

	rr := &resources.ReviewRequest{
		Platform:    "bitbucket",
		Repository:  repo,
		URL:         pr.Links.HTML.Href,
		Number:      pr.ID,
		Title:       pr.Title,
		Description: pr.Summary.Raw,
		CommitSHA:   pr.Source.Commit.SHA,
		Draft:       false, // Bitbucket Cloud does not support draft PRs.
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
		Status:      pr.State,
		Author: resources.Author{
			ID:        pr.Author.UUID,
			Login:     pr.Author.DisplayName,
			AvatarURL: pr.Author.Links.Avatar.Href,
		},
		Branch:                pr.Source.Branch.Name,
		BaseBranch:            pr.Destination.Branch.Name,
		ChangesRequestedCount: changesRequestedCount,
		ApprovedCount:         approvalCount,
		ReviewDecision:        reviewDecision,
		Reviewers:             reviewers,
		// Note(i4k): PushedAt will be set only in previews.
	}
	return rr
}

func getGithubPRByNumberOrCommit(githubClient *gh.Client, ghToken, owner, repo string, number int, commit string) (*gh.PullRequest, error) {
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
		if errors.IsKind(err, errGithubNotFound) {
			if ghToken == "" {
				logger.Warn().Msg("The GITHUB_TOKEN environment variable needs to be exported for private repositories.")
			} else {
				logger.Warn().Msg("The provided GitHub token does not have permission to read this repository or it does not exists.")
			}
		} else if errors.IsKind(err, errGithubUnprocessableEntity) {
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
		return nil, fmt.Errorf("no pull request associated with HEAD commit")
	}

	return pull, nil
}

func getGithubCommit(ghClient *gh.Client, owner, repo, commit string) (*gh.RepositoryCommit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	rcommit, _, err := ghClient.Repositories.GetCommit(ctx, owner, repo, commit, nil)
	if err != nil {
		return nil, err
	}

	return rcommit, nil
}

func getGithubPRByNumber(ghClient *gh.Client, owner string, repo string, number int) (*gh.PullRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	pull, _, err := ghClient.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}

	return pull, nil

}

// returns nil, nil if there was no PR associated with commit
func getGithubPRByCommit(ghClient *gh.Client, owner, repo, commit string) (*gh.PullRequest, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	opt := &gh.ListOptions{PerPage: 1}

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
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
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

func isGithubPRMerged(ghClient *gh.Client, owner, repo string, pullNumber int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	isMerged, _, err := ghClient.PullRequests.IsMerged(ctx, owner, repo, pullNumber)
	return isMerged, err
}

func listGithubPullReviews(ghClient *gh.Client, owner, repo string, pullNumber int) ([]*gh.PullRequestReview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	opt := &gh.ListOptions{}
	opt.PerPage = 100

	var allReviews []*gh.PullRequestReview
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

func listGithubChecks(ghClient *gh.Client, owner, repo string, commit string) ([]*gh.CheckRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultTimeout)
	defer cancel()

	opt := &gh.ListCheckRunsOptions{}
	opt.PerPage = 100

	var allChecks []*gh.CheckRun
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

func setDefaultGitMetadata(md *resources.DeploymentMetadata, commit *git.CommitMetadata) {
	md.GitCommitAuthorName = commit.Author
	md.GitCommitAuthorEmail = commit.Email
	md.GitCommitAuthorTime = commit.Time
	md.GitCommitTitle = commit.Subject
	md.GitCommitDescription = commit.Body
}

func setGithubActionsMetadata(md *resources.DeploymentMetadata) {
	md.GithubActionsDeploymentActorID = os.Getenv("GITHUB_ACTOR_ID")
	md.GithubActionsDeploymentActor = os.Getenv("GITHUB_ACTOR")
	md.GithubActionsDeploymentTriggeredBy = os.Getenv("GITHUB_TRIGGERING_ACTOR")
	md.GithubActionsDeploymentBranch = os.Getenv("GITHUB_REF_NAME")
	md.GithubActionsRunID = os.Getenv("GITHUB_RUN_ID")
	md.GithubActionsRunAttempt = os.Getenv("GITHUB_RUN_ATTEMPT")
	md.GithubActionsWorkflowName = os.Getenv("GITHUB_WORKFLOW")
	md.GithubActionsWorkflowRef = os.Getenv("GITHUB_WORKFLOW_REF")
	md.GithubActionsServerURL = os.Getenv("GITHUB_SERVER_URL")
}

func setGithubCommitMetadata(md *resources.DeploymentMetadata, commit *gh.RepositoryCommit) {
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

	// New grouping structure.
	md.GithubCommit = metadata.NewGithubCommit(commit)
}

func setGithubPRMetadata(md *resources.DeploymentMetadata, pull *gh.PullRequest) {
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

func newGithubReviewRequest(
	e *engine.Engine,
	pull *gh.PullRequest,
	reviews []*gh.PullRequestReview,
	checks []*gh.CheckRun,
	merged bool,
	reviewDecision string,
	state *CloudRunState,
) *resources.ReviewRequest {
	author := resources.Author{}
	if user := pull.GetUser(); user != nil {
		author.ID = strconv.Itoa64(int64(user.GetID()))
		author.Login = user.GetLogin()
		author.AvatarURL = user.GetAvatarURL()
	}
	pullCreatedAt := pull.GetCreatedAt()
	pullUpdatedAt := pull.GetUpdatedAt()

	repo, err := e.Project().PrettyRepo()
	if err != nil {
		e.DisableCloudFeatures(err)
		return nil
	}
	rr := &resources.ReviewRequest{
		Platform:       "github",
		Repository:     repo,
		URL:            pull.GetHTMLURL(),
		Number:         pull.GetNumber(),
		Title:          pull.GetTitle(),
		Description:    pull.GetBody(),
		CommitSHA:      pull.GetHead().GetSHA(),
		Draft:          pull.GetDraft(),
		ReviewDecision: reviewDecision,
		CreatedAt:      pullCreatedAt.GetTime(),
		UpdatedAt:      pullUpdatedAt.GetTime(),
		PushedAt:       state.RREvent.PushedAt,
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
		rr.Labels = append(rr.Labels, resources.Label{
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

		user := review.GetUser()
		if user == nil {
			continue
		}

		login := user.GetLogin()
		if _, found := uniqueReviewers[login]; found {
			continue
		}
		uniqueReviewers[login] = struct{}{}

		rr.Reviewers = append(rr.Reviewers, resources.Reviewer{
			ID:        strconv.Itoa64(user.GetID()),
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

func newGitlabReviewRequest(e *engine.Engine, mr gitlab.MR, state *CloudRunState) *resources.ReviewRequest {
	if state.RREvent.PushedAt == nil {
		panic(errors.E(errors.ErrInternal, "CI pushed_at is nil"))
	}
	mrUpdatedAt, err := time.Parse(time.RFC3339, mr.UpdatedAt)
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to parse MR.updated_at field", err)
	}
	var mrCreatedAt *time.Time
	if mrCreatedAtVal, err := time.Parse(time.RFC3339, mr.CreatedAt); err != nil {
		printer.Stderr.WarnWithDetails("failed to parse MR.created_at field", err)
	} else {
		mrCreatedAt = &mrCreatedAtVal
	}
	repo, err := e.Project().PrettyRepo()
	if err != nil {
		e.DisableCloudFeatures(err)
		return nil
	}
	rr := &resources.ReviewRequest{
		Platform:    "gitlab",
		Repository:  repo,
		URL:         mr.WebURL,
		Number:      mr.IID,
		Title:       mr.Title,
		Description: mr.Description,
		CommitSHA:   mr.SHA,
		Draft:       mr.Draft,
		CreatedAt:   mrCreatedAt,
		UpdatedAt:   &mrUpdatedAt,
		Status:      mr.State,
		Author: resources.Author{
			ID:        strconv.Itoa64(int64(mr.Author.ID)),
			Login:     mr.Author.Username,
			AvatarURL: mr.Author.AvatarURL,
		},
		Branch:     mr.SourceBranch,
		BaseBranch: mr.TargetBranch,
		// Note(i4k): PushedAt will be set only in previews.
	}

	for _, l := range mr.Labels {
		rr.Labels = append(rr.Labels, resources.Label{
			Name: l,
		})
	}

	// TODO(i4k): implement reviewers for Gitlab

	return rr
}

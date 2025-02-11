// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata

import (
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/terramate-io/terramate/strconv"
)

// GithubCommit is a Github commit.
type GithubCommit struct {
	// GithubAuthor is the Github author from the Github perspective.
	GithubAuthor *GithubUser `json:"github_author,omitempty"`

	// GitAuthor is the Git author from the Github perspective.
	GitAuthor *CommitAuthor `json:"git_author,omitempty"`

	// GithubCommitter is the Github committer from the Git perspective.
	GithubCommitter *GithubUser `json:"github_committer,omitempty"`

	// GitCommitter is the Git committer from the Git perspective.
	GitCommitter *CommitAuthor `json:"git_committer,omitempty"`
}

// GithubPullRequest is a Github pull request.
type GithubPullRequest struct {
	// Author of the PR.
	User *GithubUser `json:"user,omitempty"`
	// Author of latest commit on the PR branch.
	HeadUser *GithubUser `json:"head_user,omitempty"`
	// Author of latest commit on base branch.
	BaseUser *GithubUser `json:"base_user,omitempty"`

	RequestedReviewers []GithubUser `json:"requested_reviewers,omitempty"`
	RequestedTeams     []GithubTeam `json:"requested_teams,omitempty"`

	Reviews []GithubPullRequestReview `json:"reviews,omitempty"`

	Assignees []GithubUser `json:"assignees,omitempty"`
}

// GithubUser is a Github user.
type GithubUser struct {
	ID         string `json:"id,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
	Login      string `json:"author_login,omitempty"`
	Name       string `json:"author_gravatar_id,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	GravatarID string `json:"gravatar_id,omitempty"`
	Email      string `json:"email,omitempty"`
}

// CommitAuthor is a Github commit author.
type CommitAuthor struct {
	Name  string     `json:"name,omitempty"`
	Email string     `json:"email,omitempty"`
	Time  *time.Time `json:"time,omitempty"`
}

// GithubTeam is a Github team.
type GithubTeam struct {
	ID     string `json:"id,omitempty"`
	NodeID string `json:"node_id,omitempty"`
	Name   string `json:"name,omitempty"`
	Slug   string `json:"slug,omitempty"`
}

// GithubPullRequestReview is a Github pull request review.
type GithubPullRequestReview struct {
	ID          string      `json:"id,omitempty"`
	NodeID      string      `json:"node_id,omitempty"`
	User        *GithubUser `json:"user,omitempty"`
	SubmittedAt *time.Time  `json:"submitted_at,omitempty"`
	// State can have values CHANGES_REQUESTED, APPROVED, DISMISSED, COMMENTED
	State             string `json:"state,omitempty"`
	AuthorAssociation string `json:"author_association,omitempty"`
}

// NewGithubCommit returns a new GithubCommit.
func NewGithubCommit(in *github.RepositoryCommit) *GithubCommit {
	return &GithubCommit{
		GithubAuthor:    NewGithubUser(in.GetAuthor()),
		GitAuthor:       NewCommitAuthor(in.GetCommit().GetAuthor()),
		GithubCommitter: NewGithubUser(in.GetCommitter()),
		GitCommitter:    NewCommitAuthor(in.GetCommit().GetCommitter()),
	}
}

// NewGithubPullRequest returns a new GithubPullRequest.
func NewGithubPullRequest(inPR *github.PullRequest, inReviews []*github.PullRequestReview) *GithubPullRequest {
	requestedReviewers := make([]GithubUser, 0, len(inPR.RequestedReviewers))
	for _, e := range inPR.RequestedReviewers {
		requestedReviewers = append(requestedReviewers, *NewGithubUser(e))
	}

	requestedTeams := make([]GithubTeam, 0, len(inPR.RequestedTeams))
	for _, e := range inPR.RequestedTeams {
		requestedTeams = append(requestedTeams, *NewGithubTeam(e))
	}

	reviews := make([]GithubPullRequestReview, 0, len(inReviews))
	for _, e := range inReviews {
		reviews = append(reviews, *NewGithubPullRequestReview(e))
	}

	assignees := make([]GithubUser, 0, len(inPR.Assignees))
	for _, e := range inPR.Assignees {
		assignees = append(assignees, *NewGithubUser(e))
	}

	return &GithubPullRequest{
		User:     NewGithubUser(inPR.GetUser()),
		HeadUser: NewGithubUser(inPR.GetHead().GetUser()),
		BaseUser: NewGithubUser(inPR.GetBase().GetUser()),

		RequestedReviewers: requestedReviewers,
		RequestedTeams:     requestedTeams,

		Reviews:   reviews,
		Assignees: assignees,
	}
}

// NewCommitAuthor returns a new CommitAuthor.
func NewCommitAuthor(in *github.CommitAuthor) *CommitAuthor {
	return &CommitAuthor{
		Name:  in.GetName(),
		Email: in.GetEmail(),
	}
}

// NewGithubUser returns a new GithubUser.
func NewGithubUser(in *github.User) *GithubUser {
	return &GithubUser{
		ID:         strconv.Itoa64(in.GetID()),
		NodeID:     in.GetNodeID(),
		Login:      in.GetLogin(),
		Name:       in.GetName(),
		AvatarURL:  in.GetAvatarURL(),
		GravatarID: in.GetGravatarID(),
		Email:      in.GetEmail(),
	}
}

// NewGithubTeam returns a new GithubTeam.
func NewGithubTeam(in *github.Team) *GithubTeam {
	return &GithubTeam{
		ID:     strconv.Itoa64(in.GetID()),
		NodeID: in.GetNodeID(),
		Name:   in.GetName(),
		Slug:   in.GetSlug(),
	}
}

// NewGithubPullRequestReview returns a new GithubPullRequestReview.
func NewGithubPullRequestReview(in *github.PullRequestReview) *GithubPullRequestReview {
	return &GithubPullRequestReview{
		ID:                strconv.Itoa64(in.GetID()),
		NodeID:            in.GetNodeID(),
		User:              NewGithubUser(in.GetUser()),
		SubmittedAt:       in.SubmittedAt.GetTime(),
		State:             in.GetState(),
		AuthorAssociation: in.GetAuthorAssociation(),
	}
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v58/github"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/metadata"
)

func TestNewGithubPullRequest(t *testing.T) {
	var prs []*github.PullRequest
	var reviews []*github.PullRequestReview

	b, err := os.ReadFile("testdata/github_pr.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &prs))
	assert.EqualInts(t, 1, len(prs))

	b, err = os.ReadFile("testdata/github_pr_reviews.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &reviews))
	assert.EqualInts(t, 1, len(reviews))

	got := metadata.NewGithubPullRequest(prs[0], reviews)

	bob := metadata.GithubUser{
		ID:        "1234",
		NodeID:    "MDQ6VXNlcjg0MjA3NzUx",
		Login:     "bob",
		AvatarURL: "https://avatars.githubusercontent.com/u/1234?v=4",
	}

	otherUser := metadata.GithubUser{
		ID:        "1",
		NodeID:    "MDQ6VXNlcjE=",
		Login:     "other_user",
		AvatarURL: "https://github.com/images/error/other_user_happy.gif",
	}

	want := &metadata.GithubPullRequest{
		User:               &bob,
		HeadUser:           &bob,
		BaseUser:           &bob,
		RequestedReviewers: []metadata.GithubUser{otherUser},
		RequestedTeams: []metadata.GithubTeam{
			{ID: "1", NodeID: "MDQ6VGVhbTE=", Name: "Justice League", Slug: "justice-league"},
		},
		Assignees: []metadata.GithubUser{bob},
		Reviews: []metadata.GithubPullRequestReview{
			{
				ID:     "80",
				NodeID: "MDE3OlB1bGxSZXF1ZXN0UmV2aWV3ODA=",
				User: &metadata.GithubUser{
					ID:        "1",
					NodeID:    "MDQ6VXNlcjE=",
					Login:     "octocat",
					AvatarURL: "https://github.com/images/error/octocat_happy.gif",
				},
				SubmittedAt:       githubTimestamp(t, "2019-11-17T17:43:43Z"),
				State:             "APPROVED",
				AuthorAssociation: "COLLABORATOR",
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal("-got +want:\n" + diff)
	}
}

func TestNewGithubCommit(t *testing.T) {
	var commits []*github.RepositoryCommit

	b, err := os.ReadFile("testdata/github_commit.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &commits))
	assert.EqualInts(t, 1, len(commits))

	got := metadata.NewGithubCommit(commits[0])

	want := &metadata.GithubCommit{
		GithubAuthor: &metadata.GithubUser{
			ID:        "1",
			NodeID:    "MDQ6VXNlcjE=",
			Login:     "octocat",
			AvatarURL: "https://github.com/images/error/octocat_happy.gif",
		},
		GitAuthor: &metadata.CommitAuthor{Name: "Monalisa Octocat", Email: "support@github.com"},
		GithubCommitter: &metadata.GithubUser{
			ID:        "1",
			NodeID:    "MDQ6VXNlcjE=",
			Login:     "octocat",
			AvatarURL: "https://github.com/images/error/octocat_happy.gif",
		},
		GitCommitter: &metadata.CommitAuthor{Name: "Monalisa Octocat", Email: "support@github.com"},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal("-got +want:\n" + diff)
	}
}

func githubTimestamp(t *testing.T, s string) *time.Time {
	parsedTime, err := time.Parse(time.RFC3339, s)
	assert.NoError(t, err)
	return &parsedTime
}

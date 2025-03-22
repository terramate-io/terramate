// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/api/metadata"
	"github.com/terramate-io/terramate/cloud/integrations/gitlab"
)

func TestNewGitlabMergeRequest(t *testing.T) {
	var mrs gitlab.MRs
	var reviewers []gitlab.MRReviewer
	var participants []gitlab.User

	b, err := os.ReadFile("testdata/gitlab_mr.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &mrs))
	assert.EqualInts(t, 1, len(mrs))

	b, err = os.ReadFile("testdata/gitlab_mr_reviewers.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &reviewers))
	assert.EqualInts(t, 1, len(reviewers))

	b, err = os.ReadFile("testdata/gitlab_mr_participants.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &participants))
	assert.EqualInts(t, 1, len(participants))

	got := metadata.NewGitlabMergeRequest(&mrs[0], reviewers, participants)

	want := &metadata.GitlabMergeRequest{
		Author: &metadata.GitlabUser{
			ID:        "1",
			Username:  "bob",
			Name:      "Bob",
			WebURL:    "https://gitlab.com/bob",
			AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/1/avatar.png",
		},
		DetailedMergeStatus: "mergeable",
		Reviewers: []metadata.GitlabMergeRequestReviewer{
			{
				User: &metadata.GitlabUser{
					ID:        "1",
					Username:  "bob",
					Name:      "Bob",
					WebURL:    "https://gitlab.com/bob",
					AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/1/avatar.png",
				},
				CreatedAt: gitlabTimestamp(t, "2025-02-06T13:14:36.626Z"),
				State:     "reviewed",
			},
		},
		Assignees: []metadata.GitlabUser{
			{
				ID:        "1",
				Username:  "bob",
				Name:      "Bob",
				WebURL:    "https://gitlab.com/bob",
				AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/1/avatar.png",
			},
		},
		Participants: []metadata.GitlabUser{
			{
				ID:        "1",
				Username:  "bob",
				Name:      "Bob",
				WebURL:    "https://gitlab.com/bob",
				AvatarURL: "https://gitlab.com/uploads/-/system/user/avatar/1/avatar.png",
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal("-got +want:\n" + diff)
	}
}

func gitlabTimestamp(t *testing.T, s string) *time.Time {
	parsedTime, err := time.Parse(time.RFC3339, s)
	assert.NoError(t, err)
	return &parsedTime
}

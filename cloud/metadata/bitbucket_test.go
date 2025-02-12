// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/metadata"
	"github.com/terramate-io/terramate/cmd/terramate/cli/bitbucket"
)

func TestNewBitbucketPullRequest(t *testing.T) {
	var response bitbucket.PullRequestResponse

	b, err := os.ReadFile("testdata/bitbucket_pr.json")
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(b, &response))
	assert.EqualInts(t, 1, len(response.Values))

	got := metadata.NewBitbucketPullRequest(&response.Values[0])
	assert.NoError(t, err)

	falseVal := false
	trueVal := true

	want := &metadata.BitbucketPullRequest{
		Author: &metadata.BitbucketUser{
			UUID:        "{920a82eb-aaaa-4139-a337-83073962d5db}",
			DisplayName: "Bob",
			AccountID:   "712020:c9f5aeed-0ee0-4f02-8885-5303325c66d7",
			Nickname:    "Bob",
			AvatarURL:   "https://secure.gravatar.com/avatar/ecaf3a24654979238b684208e343fb76?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FS-2.png",
		},
		Participants: []metadata.BitbucketActor{
			{
				Type: "participant",
				User: &metadata.BitbucketUser{
					UUID:        "{920a82eb-aaaa-4139-a337-83073962d5db}",
					DisplayName: "Bob",
					AccountID:   "712020:c9f5aeed-0ee0-4f02-8885-5303325c66d7",
					Nickname:    "Bob",
					AvatarURL:   "https://secure.gravatar.com/avatar/ecaf3a24654979238b684208e343fb76?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FS-2.png",
				},
				Role:           "PARTICIPANT",
				Approved:       &trueVal,
				State:          "approved",
				ParticipatedOn: "2025-02-06T23:36:12.643152+00:00",
			},
			{
				Type: "participant",
				User: &metadata.BitbucketUser{
					UUID:        "{e255a910-aaaa-4db4-a19b-ec53fa25cd87}",
					DisplayName: "Other",
					AccountID:   "712020:b02170cf-5850-47e1-a90c-176d96c57a86",
					Nickname:    "OtherNick",
					AvatarURL:   "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/712020:b02170cf-5850-47e1-a90c-176d96c57a86/978423f4-9b79-4026-874a-4f29c4e7d679/128",
				},
				Role:     "REVIEWER",
				Approved: &falseVal,
			},
		},
		Reviewers: []metadata.BitbucketUser{
			{
				UUID:        "{e255a910-aaaa-4db4-a19b-ec53fa25cd87}",
				DisplayName: "Other",
				AccountID:   "712020:b02170cf-5850-47e1-a90c-176d96c57a86",
				Nickname:    "OtherNick",
				AvatarURL:   "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/712020:b02170cf-5850-47e1-a90c-176d96c57a86/978423f4-9b79-4026-874a-4f29c4e7d679/128",
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal("-got +want:\n" + diff)
	}
}

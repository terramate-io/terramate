// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import "testing"

const validGithubEventPath = "./testdata/event_pull_request.json"

func TestGetEventFromPath(t *testing.T) {
	event, err := getEventFromPath(validGithubEventPath)
	if err != nil {
		t.Errorf("error is not nil, expected  nil")
	}

	if event == nil {
		t.Fatal("event is nil, expected not nil")
	}

	if event.PullRequest.UpdatedAt == nil {
		t.Fatal("event.PullRequest.UpdatedAt is nil, expected not nil")
	}

	const wantUpdatedAt = "2024-02-09 12:38:32 +0000 UTC"
	if event.PullRequest.UpdatedAt.String() != wantUpdatedAt {
		t.Errorf("unexpected event.PullRequest.UpdatedAt, got: %s", event.PullRequest.UpdatedAt.String())
	}
}

func TestGetEventPRUpdatedAt(t *testing.T) {
	if updatedAt := GetEventPRUpdatedAt(validGithubEventPath); updatedAt == nil {
		t.Fatal("updatedAt is nil, expected not nil")
	}

	const wantUpdatedAt = "2024-02-09 12:38:32 +0000 UTC"
	if updatedAt := GetEventPRUpdatedAt(validGithubEventPath); updatedAt.String() != wantUpdatedAt {
		t.Errorf("updatedAt is not equal to wantUpdatedAt, got: %s", updatedAt.String())
	}
}

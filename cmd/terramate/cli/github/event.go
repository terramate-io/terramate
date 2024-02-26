// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import (
	"encoding/json"
	"os"

	"github.com/google/go-github/v58/github"
	"github.com/terramate-io/terramate/errors"
)

// GetEventPR returns the pull request event from the file at GITHUB_EVENT_PATH.
func GetEventPR() (*github.PullRequest, error) {
	githubEventPath, ok := os.LookupEnv("GITHUB_EVENT_PATH")
	if !ok {
		return nil, errors.E("missing GITHUB_EVENT_PATH")
	}

	data, err := os.ReadFile(githubEventPath)
	if err != nil {
		return nil, err
	}
	var event github.PullRequestEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}

	if event.PullRequest == nil {
		return nil, errors.E("event does not contain a pull request")
	}

	return event.PullRequest, nil
}

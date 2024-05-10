// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import (
	"encoding/json"
	"os"

	"github.com/google/go-github/v58/github"
	"github.com/terramate-io/terramate/errors"
)

// Errors for when the GitHub event cannot be read.
const (
	ErrGithubEventPathEnvNotSet errors.Kind = `environment variable "GITHUB_EVENT_PATH" not set`
	ErrGithubEventUnmarshal     errors.Kind = "failed to unmarshal github event"
	ErrGithubEventMissingPR     errors.Kind = "event does not contain a pull request"
)

// GetEventPR returns the pull request event from the file at GITHUB_EVENT_PATH.
func GetEventPR() (*github.PullRequest, error) {
	githubEventPath, ok := os.LookupEnv("GITHUB_EVENT_PATH")
	if !ok {
		return nil, errors.E(ErrGithubEventPathEnvNotSet)
	}

	data, err := os.ReadFile(githubEventPath)
	if err != nil {
		return nil, errors.E(err, "failed to read event file")
	}
	var event github.PullRequestEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, errors.E(ErrGithubEventUnmarshal, err)
	}

	if event.PullRequest == nil {
		return nil, errors.E(ErrGithubEventMissingPR)
	}

	return event.PullRequest, nil
}

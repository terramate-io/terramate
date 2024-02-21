// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import (
	"encoding/json"
	"os"
	"time"

	"github.com/terramate-io/terramate/errors"
)

type eventPullRequest struct {
	UpdatedAt *time.Time `json:"updated_at"`
}
type event struct {
	PullRequest *eventPullRequest `json:"pull_request"`
}

// GetEventPRUpdatedAt returns the updated_at field from the file at `eventPath`.
// The file is expected to be a JSON file containing the GitHub event for a pull
// request.
func GetEventPRUpdatedAt(eventPath string) (*time.Time, error) {
	event, err := getEventFromPath(eventPath)
	if err != nil {
		return nil, err
	}

	if err := event.validate(); err != nil {
		return nil, err
	}

	return event.PullRequest.UpdatedAt, nil
}

func (e *event) validate() error {
	if e == nil {
		return errors.E("missing `event` in github event")
	}

	if e.PullRequest == nil {
		return errors.E("missing `pull_request` in github event")
	}

	if e.PullRequest.UpdatedAt == nil {
		return errors.E("missing `pull_request.updated_at` in github event")
	}

	return nil
}

func getEventFromPath(eventPath string) (*event, error) {
	bytes, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, err
	}

	var parsedEvent event
	if err := json.Unmarshal(bytes, &parsedEvent); err != nil {
		return nil, err
	}

	return &parsedEvent, nil
}

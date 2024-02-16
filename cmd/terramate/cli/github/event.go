// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import (
	"encoding/json"
	"os"
	"time"
)

// GetEventPRUpdatedAt returns the updated_at field from the file at `eventPath`.
// The file is expected to be a JSON file containing the GitHub event for a pull
// request.
func GetEventPRUpdatedAt(eventPath string) *time.Time {
	event, err := getEventFromPath(eventPath)
	if err != nil {
		return nil
	}

	if event != nil {
		return event.PullRequest.UpdatedAt
	}

	return nil
}

type event struct {
	PullRequest struct {
		UpdatedAt *time.Time `json:"updated_at"`
	} `json:"pull_request"`
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

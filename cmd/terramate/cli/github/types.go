// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import "time"

type (
	// Pull represents a pull request object.
	Pull struct {
		URL       string    `json:"url"`
		Number    int       `json:"number"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`

		// rest of the fields aren't important for the cli.
	}

	// Pulls represents a list of pull objects.
	Pulls []Pull
)

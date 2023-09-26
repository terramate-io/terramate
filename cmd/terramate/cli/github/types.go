// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package github

import "time"

type (
	// Pull represents a pull request object.
	Pull struct {
		URL       string     `json:"url"`
		HTMLURL   string     `json:"html_url"`
		Number    int        `json:"number"`
		Title     string     `json:"title"`
		Body      string     `json:"body"`
		State     string     `json:"state"`
		User      User       `json:"user"`
		CreatedAt *time.Time `json:"created_at,omitempty"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
		ClosedAt  *time.Time `json:"closed_at,omitempty"`
		MergedAt  *time.Time `json:"merged_at,omitempty"`

		MergeCommitSHA string  `json:"merge_commit_sha,omitempty"`
		Head           RefInfo `json:"head"`
		Base           RefInfo `json:"base"`

		// rest of the fields aren't important for the cli.
	}

	// Commit holds information of a specific commit.
	Commit struct {
		SHA    string `json:"sha"`
		Commit struct {
			Author    GitMetadata
			Committer GitMetadata
			Message   string
		}
		Author       User
		Committer    User
		Verification struct {
			Verified bool   `json:"verified"`
			Reason   string `json:"reason"`

			// rest of the fields aren't important for the cli.
		} `json:"verification"`

		// rest of the fields aren't important for the cli.
	}

	// GitMetadata holds the commit metadata exported by Github.
	GitMetadata struct {
		Name  string     `json:"name"`
		Email string     `json:"email"`
		Date  *time.Time `json:"date,omitempty"`
	}

	// User represents the Github user.
	User struct {
		Login      string `json:"login"`
		AvatarURL  string `json:"avatar_url"`
		GravatarID string `json:"gravatar_id"`
		Type       string `json:"type"`
		SiteAdmin  bool   `json:"site_admin"`

		// rest of the fields aren't important for the cli.
	}

	// RefInfo contains metadata for the git ref (HEAD, branch, etc)
	RefInfo struct {
		Label string `json:"label"`
		Ref   string `json:"ref"`
		SHA   string `json:"sha"`
		User  User   `json:"user"`

		// rest of the fields aren't important for the cli.
	}

	// Pulls represents a list of pull objects.
	Pulls []Pull
)

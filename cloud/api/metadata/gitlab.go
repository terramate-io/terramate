// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata

import (
	"strconv"
	"time"

	"github.com/terramate-io/terramate/cloud/integrations/gitlab"
)

// GitlabMergeRequest is a Gitlab merge request.
type GitlabMergeRequest struct {
	Author *GitlabUser `json:"user,omitempty"`

	DetailedMergeStatus string `json:"detailed_merge_status,omitempty"`

	// Reviewers contains only the latest review per reviewer.
	Reviewers    []GitlabMergeRequestReviewer `json:"reviewers,omitempty"`
	Participants []GitlabUser                 `json:"participants,omitempty"`
	Assignees    []GitlabUser                 `json:"assignees,omitempty"`
}

// GitlabUser is a Gitlab user.
type GitlabUser struct {
	ID        string `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	Name      string `json:"name,omitempty"`
	WebURL    string `json:"web_url,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// GitlabMergeRequestReviewer is a Gitlab merge request reviewer.
type GitlabMergeRequestReviewer struct {
	User      *GitlabUser `json:"user,omitempty"`
	CreatedAt *time.Time  `json:"created_at,omitempty"`
	// State can contain the following values:
	// * unreviewed - reviewer is assigned but has not reviewed yet
	// * reviewed - reviewed with comments, but no approval decision yet
	// * requested_changes - reviewed with requested changes
	// * approved - reviewed and approved
	// * unapproved - previous approval was revoked
	// These cases have been tested, but there may be more potential values.
	State string `json:"state,omitempty"`
}

// NewGitlabMergeRequest is a Gitlab merge request.
func NewGitlabMergeRequest(inMR *gitlab.MR, inReviewers []gitlab.MRReviewer, inParticipants []gitlab.User) *GitlabMergeRequest {
	reviewers := make([]GitlabMergeRequestReviewer, 0, len(inReviewers))
	for _, e := range inReviewers {
		reviewers = append(reviewers, *NewGitlabMergeRequestReviewer(&e))
	}

	participants := make([]GitlabUser, 0, len(inParticipants))
	for _, e := range inParticipants {
		participants = append(participants, *NewGitlabUser(&e))
	}

	assignees := make([]GitlabUser, 0, len(inMR.Assignees))
	for _, e := range inMR.Assignees {
		assignees = append(assignees, *NewGitlabUser(&e))
	}

	return &GitlabMergeRequest{
		Author:              NewGitlabUser(&inMR.Author),
		DetailedMergeStatus: inMR.DetailedMergeStatus,
		Reviewers:           reviewers,
		Participants:        participants,
		Assignees:           assignees,
	}
}

// NewGitlabUser returns a new GitlabUser.
func NewGitlabUser(in *gitlab.User) *GitlabUser {
	return &GitlabUser{
		ID:        strconv.Itoa(in.ID),
		Username:  in.Username,
		Name:      in.Name,
		WebURL:    in.WebURL,
		AvatarURL: in.AvatarURL,
	}
}

// NewGitlabMergeRequestReviewer returns a new GitlabMergeRequestReviewer.
func NewGitlabMergeRequestReviewer(in *gitlab.MRReviewer) *GitlabMergeRequestReviewer {
	return &GitlabMergeRequestReviewer{
		User:      NewGitlabUser(in.User),
		CreatedAt: in.CreatedAt,
		State:     in.State,
	}
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package metadata

import "github.com/terramate-io/terramate/cmd/terramate/cli/bitbucket"

// BitbucketPullRequest is a Bitbuckt pull request.
type BitbucketPullRequest struct {
	Author *BitbucketUser `json:"author,omitempty"`

	// Participants contains reviewers, users that approved (assigned or not assigned), and commenters.
	Participants []BitbucketActor `json:"participants,omitempty"`

	// Reviewers contains only assigned reviewers.
	Reviewers []BitbucketUser `json:"reviewers,omitempty"`
}

// BitbucketUser is a Bitbucket user.
type BitbucketUser struct {
	UUID        string `json:"uuid,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// BitbucketActor is a Bitbucket actor.
type BitbucketActor struct {
	Type string         `json:"type,omitempty"`
	User *BitbucketUser `json:"user,omitempty"`
	// Role can be PARTICIPANT, REVIEWER.
	Role     string `json:"role,omitempty"`
	Approved *bool  `json:"approved,omitempty"`
	// State can be changes_requested, approved.
	State          string `json:"state,omitempty"`
	ParticipatedOn string `json:"participated_on,omitempty"`
}

// NewBitbucketPullRequest returns a new BitbucketPullRequest.
func NewBitbucketPullRequest(in *bitbucket.PR) *BitbucketPullRequest {
	participants := make([]BitbucketActor, 0, len(in.Participants))
	for _, e := range in.Participants {
		participants = append(participants, *NewBitbucketActor(&e))
	}

	reviewers := make([]BitbucketUser, 0, len(in.Reviewers))
	for _, e := range in.Reviewers {
		reviewers = append(reviewers, *NewBitbucketUser(&e))
	}

	return &BitbucketPullRequest{
		Author:       NewBitbucketUser(&in.Author),
		Participants: participants,
		Reviewers:    reviewers,
	}
}

// NewBitbucketUser returns a new BitbucketUser.
func NewBitbucketUser(in *bitbucket.User) *BitbucketUser {
	return &BitbucketUser{
		UUID:        in.UUID,
		DisplayName: in.DisplayName,
		AccountID:   in.AccountID,
		Nickname:    in.Nickname,
		AvatarURL:   in.Links.Avatar.Href,
	}
}

// NewBitbucketActor returns a new BitbucketActor.
func NewBitbucketActor(in *bitbucket.Actor) *BitbucketActor {
	var state string
	if t, ok := in.State.(string); ok {
		state = t
	}

	return &BitbucketActor{
		Type:           in.Type,
		User:           NewBitbucketUser(&in.User),
		Role:           in.Role,
		Approved:       in.Approved,
		State:          state,
		ParticipatedOn: in.ParticipatedOn,
	}
}

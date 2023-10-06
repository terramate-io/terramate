// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package clitest

import "github.com/terramate-io/terramate/errors"

const (
	// CloudDisablingMessage is the message displayed in the warning when disabling
	// the cloud features.
	CloudDisablingMessage = "disabling the cloud features"

	// CloudNoMembershipMessage is the message displayed when the user is not member
	// of any organization.
	CloudNoMembershipMessage = "You are not an active member of any organization"

	// CloudSyncDriftFailedMessage is the message displayed when a drift sync fails.
	CloudSyncDriftFailedMessage = "failed to sync the drift status"
)

const (
	// ErrCloud indicates the cloud feature is unprocessable for any reason.
	// Depending on the uimode (human x automation) the cli can fatal or skip the
	// cloud integration.
	ErrCloud errors.Kind = "unprocessable cloud feature"

	// ErrCloudOnboardingIncomplete indicates the onboarding process is incomplete.
	ErrCloudOnboardingIncomplete errors.Kind = "cloud commands cannot be used until onboarding is complete"

	// ErrCloudStacksWithoutID indicates that some stacks are missing the ID field.
	ErrCloudStacksWithoutID errors.Kind = "all the cloud sync features requires that selected stacks contain an ID field"

	// ErrCloudTerraformPlanFile indicates there was an error gathering the plan file details.
	ErrCloudTerraformPlanFile errors.Kind = "failed to gather drift details from plan file"

	// ErrCloudInvalidTerraformPlanFilePath indicates the plan file is not valid.
	ErrCloudInvalidTerraformPlanFilePath errors.Kind = "invalid plan file path"
)

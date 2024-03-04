// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/errors"
)

func validateMetadata(metadata *cloud.DeploymentMetadata) error {
	if metadata == nil {
		return errors.E("metadata is required")
	}
	// at least git metadata must be present
	if err := validateGitMetadata(metadata.GitMetadata); err != nil {
		return errors.E(err, "validating git metadata")
	}
	return nil
}

func validateGitMetadata(metadata cloud.GitMetadata) error {
	if metadata.GitCommitSHA == "" {
		return errors.E(`field "git_commit_sha" is required`)
	}
	if metadata.GitCommitTitle == "" {
		return errors.E(`field "git_commit_title" is required`)
	}

	// NOTE(i4k): The "git_commit_description" is not validated because it can be empty.

	if metadata.GitCommitAuthorName == "" {
		return errors.E(`field "git_commit_author_name" is required`)
	}
	if metadata.GitCommitAuthorEmail == "" {
		return errors.E(`field "git_commit_author_email" is required`)
	}
	if metadata.GitCommitAuthorTime == nil || metadata.GitCommitAuthorTime.IsZero() {
		return errors.E(`field "git_commit_author_time" is required`)
	}
	return nil
}

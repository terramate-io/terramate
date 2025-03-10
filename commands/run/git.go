// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/safeguard"
)

func gitSafeguardDefaultBranchIsReachable(engine *engine.Engine, safeguards Safeguards) error {
	safeguardsEnabled := gitSafeguardRemoteEnabled(engine, safeguards)
	logger := log.With().
		Bool("is_repository", engine.Project().IsRepo()).
		Bool("is_enabled", safeguardsEnabled).
		Logger()

	if !safeguardsEnabled {
		logger.Debug().Msg("Safeguard default-branch-is-reachable is disabled.")
		return nil
	}

	if err := checkRemoteDefaultBranchIsReachable(engine); err != nil {
		return errors.E(err, "unable to reach remote default branch")
	}
	return nil
}

func gitSafeguardRemoteEnabled(engine *engine.Engine, safeguards Safeguards) bool {
	if !engine.Project().IsGitFeaturesEnabled() || safeguards.DisableCheckGitRemote {
		return false
	}

	if safeguards.reEnabled {
		return !safeguards.DisableCheckGitRemote
	}

	cfg := engine.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	isDisabled := cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitOutOfSync)
	if isDisabled {
		return false
	}

	if engine.Project().Git.RemoteConfigured {
		return true
	}

	hasRemotes, _ := engine.Project().Git.Wrapper.HasRemotes()
	return hasRemotes
}

func checkRemoteDefaultBranchIsReachable(engine *engine.Engine) error {
	gitcfg := engine.Project().GitConfig()
	gitvals := engine.Project().Git

	remoteDesc := fmt.Sprintf("remote(%s/%s)", gitcfg.DefaultRemote, gitcfg.DefaultBranch)

	logger := log.With().
		Str("head_hash", gitvals.HeadCommit).
		Str("default_branch", remoteDesc).
		Str("default_hash", gitvals.RemoteDefaultBranchCommit).
		Logger()

	outOfDateErr := errors.E(
		ErrCurrentHeadIsOutOfDate,
		"Please update the current branch with the latest changes from the default branch.",
	)

	mergeBaseCommitID, err := gitvals.Wrapper.MergeBase(gitvals.HeadCommit, gitvals.RemoteDefaultBranchCommit)
	if err != nil {
		logger.Debug().
			Msg("A common merge-base can not be determined between HEAD and default branch")
		return outOfDateErr
	}

	if mergeBaseCommitID != gitvals.RemoteDefaultBranchCommit {
		logger.Debug().
			Str("merge_base_hash", mergeBaseCommitID).
			Msg("The default branch is not equal to the common merge-base of HEAD")
		return outOfDateErr
	}

	return nil
}

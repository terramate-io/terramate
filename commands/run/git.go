// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/safeguard"
)

// GitSafeguardDefaultBranchIsReachable checks if the default branch is reachable.
func GitSafeguardDefaultBranchIsReachable(engine *engine.Engine, safeguards Safeguards) error {
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

// GitFileSafeguards checks for untracked and uncommitted files in the repository.
func GitFileSafeguards(e *engine.Engine, shouldError bool, sf Safeguards) error {
	repochecks := e.RepoChecks()
	debugFiles(repochecks.UntrackedFiles, "untracked file")
	debugFiles(repochecks.UncommittedFiles, "uncommitted file")

	if checkGitUntracked(e, sf) && len(repochecks.UntrackedFiles) > 0 {
		const msg = "repository has untracked files"
		if shouldError {
			return errors.E(msg)
		}
		log.Warn().Msg(msg)
	}

	if checkGitUncommited(e, sf) && len(repochecks.UncommittedFiles) > 0 {
		const msg = "repository has uncommitted files"
		if shouldError {
			return errors.E(msg)
		}
		log.Warn().Msg(msg)
	}
	return nil
}

func checkGitUntracked(e *engine.Engine, sf Safeguards) bool {
	if !e.Project().IsGitFeaturesEnabled() || sf.DisableCheckGitUntracked {
		return false
	}

	if sf.ReEnabled {
		return !sf.DisableCheckGitUntracked
	}

	cfg := e.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUntracked)
}

func checkGitUncommited(e *engine.Engine, sf Safeguards) bool {
	if !e.Project().IsGitFeaturesEnabled() || sf.DisableCheckGitUncommitted {
		return false
	}

	if sf.ReEnabled {
		return !sf.DisableCheckGitUncommitted
	}

	cfg := e.RootNode()
	if cfg.Terramate == nil || cfg.Terramate.Config == nil {
		return true
	}
	return !cfg.Terramate.Config.HasSafeguardDisabled(safeguard.GitUncommitted)
}

func debugFiles(files project.Paths, msg string) {
	for _, file := range files {
		log.Debug().
			Stringer("file", file).
			Msg(msg)
	}
}

func gitSafeguardRemoteEnabled(engine *engine.Engine, safeguards Safeguards) bool {
	if !engine.Project().IsGitFeaturesEnabled() || safeguards.DisableCheckGitRemote {
		return false
	}

	if safeguards.ReEnabled {
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
	prj := engine.Project()
	gitcfg := prj.GitConfig()

	remoteDesc := fmt.Sprintf("remote(%s/%s)", gitcfg.DefaultRemote, gitcfg.DefaultBranch)

	logger := log.With().
		Str("default_branch", remoteDesc).
		Logger()

	outOfDateErr := errors.E(
		ErrCurrentHeadIsOutOfDate,
		"Please update the current branch with the latest changes from the default branch.",
	)

	headCommit, err1 := prj.HeadCommit()
	remoteDefaultBranchCommit, err2 := prj.RemoteDefaultCommit()
	if err := errors.L(err1, err2); err.AsError() != nil {
		return err
	}

	mergeBaseCommitID, err := prj.Git.Wrapper.MergeBase(headCommit, remoteDefaultBranchCommit)
	if err != nil {
		logger.Debug().
			Msg("A common merge-base can not be determined between HEAD and default branch")
		return outOfDateErr
	}

	if mergeBaseCommitID != remoteDefaultBranchCommit {
		logger.Debug().
			Str("merge_base_hash", mergeBaseCommitID).
			Msg("The default branch is not equal to the common merge-base of HEAD")
		return outOfDateErr
	}

	return nil
}

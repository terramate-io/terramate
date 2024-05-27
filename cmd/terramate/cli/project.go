// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/stack"
)

type project struct {
	rootdir        string
	wd             string
	isRepo         bool
	root           config.Root
	baseRev        string
	normalizedRepo string
	stackManager   *stack.Manager

	git struct {
		wrapper                   *git.Git
		headCommit                string
		localDefaultBranchCommit  string
		remoteDefaultBranchCommit string

		remoteConfigured bool
		branchConfigured bool

		repoChecks stack.RepoChecks
	}
}

func (p project) gitcfg() *hcl.GitConfig {
	return p.root.Tree().Node.Terramate.Config.Git
}

func (p *project) prettyRepo() string {
	if p.normalizedRepo != "" {
		return p.normalizedRepo
	}
	if p.isRepo {
		repoURL, err := p.git.wrapper.URL(p.gitcfg().DefaultRemote)
		if err == nil {
			p.normalizedRepo = cloud.NormalizeGitURI(repoURL)
		} else {
			logger := log.With().
				Str("action", "project.prettyRepo").
				Logger()

			logger.
				Warn().
				Err(err).
				Msg("failed to retrieve repository URL")
		}
	}
	return p.normalizedRepo
}

func (p *project) isGitFeaturesEnabled() bool {
	return p.isRepo && p.hasCommit()
}

func (p *project) hasCommit() bool {
	_, err := p.git.wrapper.RevParse("HEAD")
	return err == nil
}

func (p *project) hasCommits() bool {
	_, err := p.git.wrapper.RevParse("HEAD^")
	return err == nil
}

func (p *project) headCommit() string {
	if p.git.headCommit != "" {
		return p.git.headCommit
	}

	val, err := p.git.wrapper.RevParse("HEAD")
	if err != nil {
		fatalWithDetails(err, "unable to git rev-parse")
	}

	p.git.headCommit = val
	return val
}

func (p *project) remoteDefaultCommit() string {
	if p.git.remoteDefaultBranchCommit != "" {
		return p.git.remoteDefaultBranchCommit
	}

	gitcfg := p.gitcfg()
	remoteRef, err := p.git.wrapper.FetchRemoteRev(gitcfg.DefaultRemote, gitcfg.DefaultBranch)
	if err != nil {
		fatalWithDetails(
			fmt.Errorf("fetching remote commit of %s/%s: %v",
				gitcfg.DefaultRemote, gitcfg.DefaultBranch,
				err,
			),
			"unable to fetch remote commit")
	}

	p.git.remoteDefaultBranchCommit = remoteRef.CommitID
	return p.git.remoteDefaultBranchCommit
}

// selectChangeBase returns the revision used for change comparison based on the current Git state.
func (p *project) selectChangeBase() string {
	gitcfg := p.gitcfg()
	gw := p.git.wrapper

	// Try using remote default branch first
	defaultBranchRev, _ := gw.RevParse(gitcfg.DefaultRemote + "/" + gitcfg.DefaultBranch)
	if defaultBranchRev == "" {
		// Fall back to local default branch
		defaultBranchRev, _ = gw.RevParse(gitcfg.DefaultBranch)

		if defaultBranchRev == "" {
			// There's no default branch available, so we can't look for a common parent with it.
			return defaultBranchBaseRef
		}
	}

	branch, _ := gw.CurrentBranch()

	// Either we are on a branch or at a detached HEAD.
	if branch != "" {
		if branch == gitcfg.DefaultBranch {
			// We are at the tip of the default branch -> latest default commit.
			return defaultBranchBaseRef
		}

		// Fallthrough to common parent if not on default branch
	} else {
		headRev, _ := gw.RevParse("HEAD")
		isDetachedDefaultBranchTip := headRev == defaultBranchRev
		if isDetachedDefaultBranchTip {
			// We are at the latest commit of the default branch.
			return defaultBranchBaseRef
		}

		isDefaultBranchAncestor, _ := gw.IsFirstParentAncestor("HEAD", defaultBranchRev)
		if isDefaultBranchAncestor {
			// We are at an older commit of the default branch.
			return defaultBranchBaseRef
		}

		// Fallthrough to common parent if not at commit of default branch
	}

	commonParentWithDefaultBranch, _ := gw.FindNearestCommonParent(defaultBranchRev, "HEAD")
	if commonParentWithDefaultBranch != "" {
		// We have a nearest common parent with the default branch. Similar to the historic merge base.
		return commonParentWithDefaultBranch
	}

	// Fall back to default. Should never happen unless running on an isolated commit.
	return defaultBranchBaseRef
}

func (p *project) setDefaults() error {
	logger := log.With().
		Str("action", "setDefaults()").
		Str("workingDir", p.wd).
		Logger()

	logger.Debug().Msg("Set defaults.")

	cfg := &p.root.Tree().Node
	if cfg.Terramate == nil {
		// if config has no terramate block we create one with default
		// configurations.
		cfg.Terramate = &hcl.Terramate{}
	}

	if cfg.Terramate.Config == nil {
		cfg.Terramate.Config = &hcl.RootConfig{}
	}
	// Now some defaults are defined on the NewGitConfig but others here.
	// To define all defaults here we would need boolean pointers to
	// check if the config is defined or not, the zero value for booleans
	// is valid (simpler with strings). Maybe we could move all defaults
	// to NewGitConfig.
	if cfg.Terramate.Config.Git == nil {
		cfg.Terramate.Config.Git = hcl.NewGitConfig()
	}

	gitOpt := cfg.Terramate.Config.Git

	p.git.branchConfigured = gitOpt.DefaultBranch != ""
	if !p.git.branchConfigured {
		gitOpt.DefaultBranch = defaultBranch
	}

	p.git.remoteConfigured = gitOpt.DefaultRemote != ""
	if !p.git.remoteConfigured {
		gitOpt.DefaultRemote = defaultRemote
	}

	return nil
}

func (p project) checkDefaultRemote() error {
	remotes, err := p.git.wrapper.Remotes()
	if err != nil {
		return fmt.Errorf("checking if remote %q exists: %v", defaultRemote, err)
	}

	var defRemote *git.Remote

	gitcfg := p.gitcfg()

	for _, remote := range remotes {
		if remote.Name == gitcfg.DefaultRemote {
			remote := remote
			defRemote = &remote
			break
		}
	}

	if defRemote == nil {
		return fmt.Errorf("repository must have a configured %q remote",
			gitcfg.DefaultRemote,
		)
	}

	for _, branch := range defRemote.Branches {
		if branch == gitcfg.DefaultBranch {
			return nil
		}
	}

	return fmt.Errorf("remote %q has no default branch %q,branches:%v",
		gitcfg.DefaultRemote,
		gitcfg.DefaultBranch,
		defRemote.Branches,
	)
}

func (p *project) checkRemoteDefaultBranchIsReachable() error {
	gitcfg := p.gitcfg()

	remoteDesc := fmt.Sprintf("remote(%s/%s)", gitcfg.DefaultRemote, gitcfg.DefaultBranch)

	logger := log.With().
		Str("head_hash", p.headCommit()).
		Str("default_branch", remoteDesc).
		Str("default_hash", p.remoteDefaultCommit()).
		Logger()

	outOfDateErr := errors.E(
		ErrCurrentHeadIsOutOfDate,
		"Please update the current branch with the latest changes from the default branch.",
	)

	mergeBaseCommitID, err := p.git.wrapper.MergeBase(p.headCommit(), p.remoteDefaultCommit())
	if err != nil {
		logger.Debug().
			Msg("A common merge-base can not be determined between HEAD and default branch")
		return outOfDateErr
	}

	if mergeBaseCommitID != p.remoteDefaultCommit() {
		logger.Debug().
			Str("merge_base_hash", mergeBaseCommitID).
			Msg("The default branch is not equal to the common merge-base of HEAD")
		return outOfDateErr
	}

	return nil
}

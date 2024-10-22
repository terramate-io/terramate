// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/stack"
)

type project struct {
	rootdir      string
	wd           string
	isRepo       bool
	root         config.Root
	baseRef      string
	repository   *git.Repository
	stackManager *stack.Manager

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

func (p *project) repo() (*git.Repository, error) {
	if !p.isRepo {
		panic(errors.E(errors.ErrInternal, "called without a repository"))
	}
	if p.repository != nil {
		return p.repository, nil
	}
	repoURL, err := p.git.wrapper.URL(p.gitcfg().DefaultRemote)
	if err != nil {
		return nil, err
	}
	r, err := git.NormalizeGitURI(repoURL)
	if err != nil {
		return nil, err
	}
	p.repository = &r
	return p.repository, nil
}

func (p *project) prettyRepo() string {
	r, err := p.repo()
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to retrieve repository URL", err)
		return "<invalid>"
	}
	return r.Repo
}

func (p *project) localDefaultBranchCommit() string {
	if p.git.localDefaultBranchCommit != "" {
		return p.git.localDefaultBranchCommit
	}
	gitcfg := p.gitcfg()
	refName := gitcfg.DefaultRemote + "/" + gitcfg.DefaultBranch
	val, err := p.git.wrapper.RevParse(refName)
	if err != nil {
		fatalWithDetailf(err, "unable to git rev-parse")
	}

	p.git.localDefaultBranchCommit = val
	return val
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
		fatalWithDetailf(err, "unable to git rev-parse")
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
		fatalWithDetailf(
			fmt.Errorf("fetching remote commit of %s/%s: %v",
				gitcfg.DefaultRemote, gitcfg.DefaultBranch,
				err,
			),
			"unable to fetch remote commit")
	}

	p.git.remoteDefaultBranchCommit = remoteRef.CommitID
	return p.git.remoteDefaultBranchCommit
}

func (p *project) isDefaultBranch() bool {
	git := p.gitcfg()
	branch, err := p.git.wrapper.CurrentBranch()
	if err != nil {
		// WHY?
		// The current branch name (the symbolic-ref of the HEAD) is not always
		// available, in this case we naively check if HEAD == local origin/main.
		// This case usually happens in the git setup of CIs.
		return p.localDefaultBranchCommit() == p.headCommit()
	}

	return branch == git.DefaultBranch
}

// defaultBaseRef returns the baseRef for the current git environment.
func (p *project) defaultBaseRef() string {
	if p.isDefaultBranch() &&
		p.remoteDefaultCommit() == p.headCommit() {
		_, err := p.git.wrapper.RevParse(defaultBranchBaseRef)
		if err == nil {
			return defaultBranchBaseRef
		}
	}
	return p.defaultBranchRef()
}

// defaultLocalBaseRef returns the baseRef in case there's no remote setup.
func (p *project) defaultLocalBaseRef() string {
	git := p.gitcfg()
	if p.isDefaultBranch() {
		_, err := p.git.wrapper.RevParse(defaultBranchBaseRef)
		if err == nil {
			return defaultBranchBaseRef
		}
	}
	return git.DefaultBranch
}

func (p project) defaultBranchRef() string {
	git := p.gitcfg()
	return git.DefaultRemote + "/" + git.DefaultBranch
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

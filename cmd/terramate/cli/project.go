// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"fmt"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
)

type project struct {
	root    string
	wd      string
	isRepo  bool
	cfg     config.Tree
	baseRef string

	git struct {
		wrapper                   *git.Git
		headCommit                string
		localDefaultBranchCommit  string
		remoteDefaultBranchCommit string
	}
}

func (p project) gitcfg() *hcl.GitConfig {
	return p.cfg.Node.Terramate.Config.Git
}

func (p *project) localDefaultBranchCommit() string {
	if p.git.localDefaultBranchCommit != "" {
		return p.git.localDefaultBranchCommit
	}
	logger := log.With().
		Str("action", "localDefaultBranchCommit()").
		Logger()

	gitcfg := p.gitcfg()
	refName := gitcfg.DefaultRemote + "/" + gitcfg.DefaultBranch
	val, err := p.git.wrapper.RevParse(refName)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	p.git.localDefaultBranchCommit = val
	return val
}

func (p *project) headCommit() string {
	if p.git.headCommit != "" {
		return p.git.headCommit
	}

	logger := log.With().
		Str("action", "headCommit()").
		Logger()

	val, err := p.git.wrapper.RevParse("HEAD")
	if err != nil {
		logger.Fatal().Err(err).Send()
	}

	p.git.headCommit = val
	return val
}

func (p *project) remoteDefaultCommit() string {
	if p.git.remoteDefaultBranchCommit != "" {
		return p.git.remoteDefaultBranchCommit
	}

	logger := log.With().
		Str("action", "remoteDefaultCommit()").
		Logger()

	gitcfg := p.gitcfg()
	remoteRef, err := p.git.wrapper.FetchRemoteRev(gitcfg.DefaultRemote, gitcfg.DefaultBranch)
	if err != nil {
		logger.Fatal().Err(
			fmt.Errorf("fetching remote commit of %s/%s: %v",
				gitcfg.DefaultRemote, gitcfg.DefaultBranch,
				err,
			)).Send()
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
	git := p.gitcfg()
	if p.isDefaultBranch() &&
		p.remoteDefaultCommit() == p.headCommit() {
		_, err := p.git.wrapper.RevParse(git.DefaultBranchBaseRef)
		if err == nil {
			return git.DefaultBranchBaseRef
		}
	}

	return p.defaultBranchRef()
}

func (p project) defaultBranchRef() string {
	git := p.gitcfg()
	return git.DefaultRemote + "/" + git.DefaultBranch
}

func (p *project) setDefaults(parsedArgs *cliSpec) error {
	logger := log.With().
		Str("action", "setDefaults()").
		Str("workingDir", p.wd).
		Logger()

	logger.Debug().Msg("Set defaults.")

	if p.cfg.Node.Terramate == nil {
		// if config has no terramate block we create one with default
		// configurations.
		p.cfg.Node.Terramate = &hcl.Terramate{}
	}

	cfg := &p.cfg
	if cfg.Node.Terramate.Config == nil {
		p.cfg.Node.Terramate.Config = &hcl.RootConfig{}
	}
	// Now some defaults are defined on the NewGitConfig but others here.
	// To define all defaults here we would need boolean pointers to
	// check if the config is defined or not, the zero value for booleans
	// is valid (simpler with strings). Maybe we could move all defaults
	// to NewGitConfig.
	if cfg.Node.Terramate.Config.Git == nil {
		cfg.Node.Terramate.Config.Git = hcl.NewGitConfig()
	}

	gitOpt := cfg.Node.Terramate.Config.Git

	if gitOpt.DefaultBranchBaseRef == "" {
		gitOpt.DefaultBranchBaseRef = defaultBranchBaseRef
	}

	if gitOpt.DefaultBranch == "" {
		gitOpt.DefaultBranch = defaultBranch
	}

	if gitOpt.DefaultRemote == "" {
		gitOpt.DefaultRemote = defaultRemote
	}

	return nil
}

func (p project) checkDefaultRemote() error {
	logger := log.With().
		Str("action", "checkDefaultRemote()").
		Logger()

	logger.Trace().Msg("Get list of configured git remotes.")

	remotes, err := p.git.wrapper.Remotes()
	if err != nil {
		return fmt.Errorf("checking if remote %q exists: %v", defaultRemote, err)
	}

	var defRemote *git.Remote

	gitcfg := p.gitcfg()

	logger.Trace().
		Msg("Find default git remote.")
	for _, remote := range remotes {
		if remote.Name == gitcfg.DefaultRemote {
			defRemote = &remote
			break
		}
	}

	if defRemote == nil {
		return fmt.Errorf("repository must have a configured %q remote",
			gitcfg.DefaultRemote,
		)
	}

	logger.Trace().Msg("Find default git branch.")
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
		Str("HEAD", p.headCommit()).
		Str(remoteDesc, p.remoteDefaultCommit()).
		Logger()

	commonErr := errors.E(
		ErrCurrentHeadIsOutOfDate,
		"Please merge the latest changes from %s into this branch",
		remoteDesc,
	)

	mergeBaseCommitID, err := p.git.wrapper.MergeBase(p.headCommit(), p.remoteDefaultCommit())
	if err != nil {
		logger.Debug().Msgf("%s: %v", ErrCurrentHeadIsOutOfDate, err)
		return commonErr
	}

	logger = logger.With().
		Str("merge_base", mergeBaseCommitID).
		Logger()

	if mergeBaseCommitID != p.remoteDefaultCommit() {
		logger.Debug().Msgf("%s: The %s is not the merge-base of current HEAD",
			ErrCurrentHeadIsOutOfDate, remoteDesc)
		return commonErr
	}

	return nil
}

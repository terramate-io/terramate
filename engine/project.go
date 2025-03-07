// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/ci"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/stack"
)

const (
	defaultRemote        = "origin"
	defaultBranch        = "main"
	defaultBranchBaseRef = "HEAD^"
)

const (
	// ErrCurrentHeadIsOutOfDate indicates the local HEAD revision is outdated.
	ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch"
)

type Project struct {
	rootdir      string
	wd           string
	isRepo       bool
	root         *config.Root
	baseRef      string
	repository   *git.Repository
	platform     *ci.PlatformType
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

func NewProject(wd string) (prj *Project, found bool, err error) {
	prj = &Project{
		wd: wd,
	}

	var gitdir string
	gw, err := newGit(wd)
	if err == nil {
		gitdir, err = gw.Root()
	}
	if err == nil {
		gitabs := gitdir
		if !filepath.IsAbs(gitabs) {
			gitabs = filepath.Join(wd, gitdir)
		}

		rootdir, err := filepath.EvalSymlinks(gitabs)
		if err != nil {
			return nil, false, errors.E(err, "failed evaluating symlinks of %q", gitabs)
		}

		cfg, err := config.LoadRoot(rootdir)
		if err != nil {
			return nil, false, err
		}

		gw = gw.With().WorkingDir(rootdir).Wrapper()

		prj.isRepo = true
		prj.root = cfg
		prj.rootdir = rootdir
		prj.git.wrapper = gw

		mgr := stack.NewGitAwareManager(prj.root, gw)
		prj.stackManager = mgr

		return prj, true, nil
	}

	rootcfg, rootcfgpath, rootfound, err := config.TryLoadConfig(wd)
	if err != nil {
		return nil, false, err
	}
	if !rootfound {
		return nil, false, nil
	}
	prj.rootdir = rootcfgpath
	prj.root = rootcfg
	prj.stackManager = stack.NewManager(prj.root)
	return prj, true, nil
}

func (p *Project) IsRepo() bool { return p.isRepo }

func newGit(basedir string) (*git.Git, error) {
	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
		Env:        os.Environ(),
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (p *Project) gitcfg() *hcl.GitConfig {
	return p.root.Tree().Node.Terramate.Config.Git
}

func (p *Project) Repo() (*git.Repository, error) {
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

func (p *Project) CIPlatform() ci.PlatformType {
	if p.platform != nil {
		return *p.platform
	}
	platform := ci.DetectPlatformFromEnv()
	p.platform = &platform
	return platform
}

func (p *Project) prettyRepo() string {
	r, err := p.Repo()
	if err != nil {
		printer.Stderr.WarnWithDetails("failed to retrieve repository URL", err)
		return "<invalid>"
	}
	return r.Repo
}

func (p *Project) setupGitValues() error {
	errs := errors.L()
	for _, f := range []func() error{
		p.computeHeadCommit,
		p.computeLocalDefaultBranchCommit,
		p.computeRemoteDefaultCommit,
	} {
		errs.Append(f())
	}
	return errs.AsError()
}

func (p *Project) computeHeadCommit() error {
	if p.git.headCommit != "" {
		return nil
	}

	val, err := p.git.wrapper.RevParse("HEAD")
	if err != nil {
		return errors.E(err, "unable to git rev-parse")
	}

	p.git.headCommit = val
	return nil
}

func (p *Project) computeLocalDefaultBranchCommit() error {
	if p.git.localDefaultBranchCommit != "" {
		return nil
	}
	gitcfg := p.gitcfg()
	refName := gitcfg.DefaultRemote + "/" + gitcfg.DefaultBranch
	val, err := p.git.wrapper.RevParse(refName)
	if err != nil {
		return errors.E(err, "unable to git rev-parse")
	}

	p.git.localDefaultBranchCommit = val
	return nil
}

func (p *Project) computeRemoteDefaultCommit() error {
	if p.git.remoteDefaultBranchCommit != "" {
		return nil
	}

	gitcfg := p.gitcfg()
	remoteRef, err := p.git.wrapper.FetchRemoteRev(gitcfg.DefaultRemote, gitcfg.DefaultBranch)
	if err != nil {
		return errors.E(
			fmt.Errorf("fetching remote commit of %s/%s: %v",
				gitcfg.DefaultRemote, gitcfg.DefaultBranch,
				err,
			),
			"unable to fetch remote commit")
	}
	p.git.remoteDefaultBranchCommit = remoteRef.CommitID
	return nil
}

func (p *Project) isGitFeaturesEnabled() bool {
	return p.isRepo && p.hasCommit()
}

func (p *Project) hasCommit() bool {
	_, err := p.git.wrapper.RevParse("HEAD")
	return err == nil
}

func (p *Project) hasCommits() bool {
	_, err := p.git.wrapper.RevParse("HEAD^")
	return err == nil
}

func (p *Project) isDefaultBranch() bool {
	git := p.gitcfg()
	branch, err := p.git.wrapper.CurrentBranch()
	if err != nil {
		// WHY?
		// The current branch name (the symbolic-ref of the HEAD) is not always
		// available, in this case we naively check if HEAD == local origin/main.
		// This case usually happens in the git setup of CIs.
		return p.git.localDefaultBranchCommit == p.git.headCommit
	}

	return branch == git.DefaultBranch
}

// defaultBaseRef returns the baseRef for the current git environment.
func (p *Project) defaultBaseRef() string {
	if p.isDefaultBranch() &&
		p.git.remoteDefaultBranchCommit == p.git.headCommit {
		_, err := p.git.wrapper.RevParse(defaultBranchBaseRef)
		if err == nil {
			return defaultBranchBaseRef
		}
	}
	return p.defaultBranchRef()
}

// defaultLocalBaseRef returns the baseRef in case there's no remote setup.
func (p *Project) defaultLocalBaseRef() string {
	git := p.gitcfg()
	if p.isDefaultBranch() {
		_, err := p.git.wrapper.RevParse(defaultBranchBaseRef)
		if err == nil {
			return defaultBranchBaseRef
		}
	}
	return git.DefaultBranch
}

func (p Project) defaultBranchRef() string {
	git := p.gitcfg()
	return git.DefaultRemote + "/" + git.DefaultBranch
}

func (p *Project) setDefaults() error {
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

func (p Project) checkDefaultRemote() error {
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

func (p *Project) checkRemoteDefaultBranchIsReachable() error {
	gitcfg := p.gitcfg()

	remoteDesc := fmt.Sprintf("remote(%s/%s)", gitcfg.DefaultRemote, gitcfg.DefaultBranch)

	logger := log.With().
		Str("head_hash", p.git.headCommit).
		Str("default_branch", remoteDesc).
		Str("default_hash", p.git.remoteDefaultBranchCommit).
		Logger()

	outOfDateErr := errors.E(
		ErrCurrentHeadIsOutOfDate,
		"Please update the current branch with the latest changes from the default branch.",
	)
	mergeBaseCommitID, err := p.git.wrapper.MergeBase(p.git.headCommit, p.git.remoteDefaultBranchCommit)
	if err != nil {
		logger.Debug().
			Msg("A common merge-base can not be determined between HEAD and default branch")
		return outOfDateErr
	}
	if mergeBaseCommitID != p.git.remoteDefaultBranchCommit {
		logger.Debug().
			Str("merge_base_hash", mergeBaseCommitID).
			Msg("The default branch is not equal to the common merge-base of HEAD")
		return outOfDateErr
	}
	return nil
}

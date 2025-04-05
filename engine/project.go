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

// Project represents a Terramate project.
type Project struct {
	rootdir      string
	wd           string
	isRepo       bool
	root         *config.Root
	baseRef      string
	repository   *git.Repository
	platform     *ci.PlatformType
	stackManager *stack.Manager

	Git struct {
		Wrapper                   *git.Git
		headCommit                string
		localDefaultBranchCommit  string
		remoteDefaultBranchCommit string

		defaultBaseRef      string
		defaultLocalBaseRef string

		RemoteConfigured bool
		BranchConfigured bool
	}
}

// NewProject creates a new project from the working directory.
func NewProject(wd string, parserOpts ...hcl.Option) (prj *Project, found bool, err error) {
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

		cfg, err := config.LoadRoot(rootdir, parserOpts...)
		if err != nil {
			return nil, false, err
		}

		gw = gw.With().WorkingDir(rootdir).Wrapper()

		prj.isRepo = true
		prj.root = cfg
		prj.rootdir = rootdir
		prj.Git.Wrapper = gw

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

// IsRepo returns true if the project is a git repository.
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

// GitConfig returns the git configuration of the project.
func (p *Project) GitConfig() *hcl.GitConfig {
	return p.root.Tree().Node.Terramate.Config.Git
}

// Repo returns the git repository of the project.
func (p *Project) Repo() (*git.Repository, error) {
	if !p.isRepo {
		panic(errors.E(errors.ErrInternal, "called without a repository"))
	}
	if p.repository != nil {
		return p.repository, nil
	}
	repoURL, err := p.Git.Wrapper.URL(p.GitConfig().DefaultRemote)
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

// CIPlatform returns the CI platform of the project.
func (p *Project) CIPlatform() ci.PlatformType {
	if p.platform != nil {
		return *p.platform
	}
	platform := ci.DetectPlatformFromEnv()
	p.platform = &platform
	return platform
}

// PrettyRepo returns the pretty repository name.
func (p *Project) PrettyRepo() (string, error) {
	r, err := p.Repo()
	if err != nil {
		return "", err
	}
	return r.Repo, nil
}

// HeadCommit returns the HEAD commit of the project.
func (p *Project) HeadCommit() (string, error) {
	if p.Git.headCommit != "" {
		return p.Git.headCommit, nil
	}

	val, err := p.Git.Wrapper.RevParse("HEAD")
	if err != nil {
		return "", errors.E(err, "computing HEAD commit SHA")
	}

	p.Git.headCommit = val
	return p.Git.headCommit, nil
}

// LocalDefaultBranchCommit returns the local default branch commit of the project.
func (p *Project) LocalDefaultBranchCommit() (string, error) {
	if p.Git.localDefaultBranchCommit != "" {
		return p.Git.localDefaultBranchCommit, nil
	}
	gitcfg := p.GitConfig()
	refName := gitcfg.DefaultRemote + "/" + gitcfg.DefaultBranch
	val, err := p.Git.Wrapper.RevParse(refName)
	if err != nil {
		return "", errors.E(err, "computing local default branch commit")
	}

	p.Git.localDefaultBranchCommit = val
	return p.Git.localDefaultBranchCommit, nil
}

// RemoteDefaultCommit returns the remote default branch commit of the project.
// Eg.: git fetch origin main && git rev-parse origin/main
func (p *Project) RemoteDefaultCommit() (string, error) {
	if p.Git.remoteDefaultBranchCommit != "" {
		return p.Git.remoteDefaultBranchCommit, nil
	}

	gitcfg := p.GitConfig()
	remoteRef, err := p.Git.Wrapper.FetchRemoteRev(gitcfg.DefaultRemote, gitcfg.DefaultBranch)
	if err != nil {
		return "", errors.E(
			fmt.Errorf("fetching remote commit of %s/%s: %v",
				gitcfg.DefaultRemote, gitcfg.DefaultBranch,
				err,
			),
			"unable to fetch remote commit")
	}
	p.Git.remoteDefaultBranchCommit = remoteRef.CommitID
	return p.Git.remoteDefaultBranchCommit, nil
}

// IsGitFeaturesEnabled returns true if the project has git features enabled.
func (p *Project) IsGitFeaturesEnabled() bool {
	return p.isRepo && p.HasCommit()
}

// HasCommit returns true if the project has a commit.
func (p *Project) HasCommit() bool {
	_, err := p.Git.Wrapper.RevParse("HEAD")
	return err == nil
}

// HasCommits returns true if the project has commits.
func (p *Project) HasCommits() bool {
	_, err := p.Git.Wrapper.RevParse("HEAD^")
	return err == nil
}

func (p *Project) isDefaultBranch() (bool, error) {
	git := p.GitConfig()
	branch, err := p.Git.Wrapper.CurrentBranch()
	if err != nil {
		// WHY?
		// The current branch name (the symbolic-ref of the HEAD) is not always
		// available, in this case we naively check if HEAD == local origin/main.
		// This case usually happens in the git setup of CIs.
		errs := errors.L()
		headcommit, err := p.HeadCommit()
		errs.Append(err)
		localdefault, err := p.LocalDefaultBranchCommit()
		errs.Append(err)
		if errs.AsError() != nil {
			return false, errs
		}
		return localdefault == headcommit, nil
	}

	return branch == git.DefaultBranch, nil
}

// DefaultBaseRef returns the baseRef for the current git environment.
func (p *Project) DefaultBaseRef() (string, error) {
	if p.Git.defaultBaseRef != "" {
		return p.Git.defaultBaseRef, nil
	}
	isDefault, err := p.isDefaultBranch()
	if err != nil {
		return "", err
	}

	if isDefault {
		remoteDefault, err1 := p.RemoteDefaultCommit()
		headCommit, err2 := p.HeadCommit()
		if err := errors.L(err1, err2); err.AsError() != nil {
			return "", err
		}
		if remoteDefault == headCommit {
			_, err := p.Git.Wrapper.RevParse(defaultBranchBaseRef)
			if err == nil {
				p.Git.defaultBaseRef = defaultBranchBaseRef
				return p.Git.defaultBaseRef, nil
			}
		}
	}
	p.Git.defaultBaseRef = p.DefaultBranchRef()
	return p.Git.defaultBaseRef, nil
}

// DefaultLocalBaseRef returns the baseRef in case there's no remote setup.
func (p *Project) DefaultLocalBaseRef() (string, error) {
	if p.Git.defaultLocalBaseRef != "" {
		return p.Git.defaultLocalBaseRef, nil
	}
	git := p.GitConfig()
	isDefault, err := p.isDefaultBranch()
	if err != nil {
		return "", err
	}

	if isDefault {
		_, err := p.Git.Wrapper.RevParse(defaultBranchBaseRef)
		if err == nil {
			p.Git.defaultLocalBaseRef = defaultBranchBaseRef
			return p.Git.defaultLocalBaseRef, nil
		}
	}
	p.Git.defaultLocalBaseRef = git.DefaultBranch
	return p.Git.defaultLocalBaseRef, nil
}

// DefaultBranchRef returns the default branch ref.
// Usually it's "origin/main".
func (p Project) DefaultBranchRef() string {
	git := p.GitConfig()
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

	p.Git.BranchConfigured = gitOpt.DefaultBranch != ""
	if !p.Git.BranchConfigured {
		gitOpt.DefaultBranch = defaultBranch
	}

	p.Git.RemoteConfigured = gitOpt.DefaultRemote != ""
	if !p.Git.RemoteConfigured {
		gitOpt.DefaultRemote = defaultRemote
	}
	return nil
}

func (p Project) checkDefaultRemote() error {
	remotes, err := p.Git.Wrapper.Remotes()
	if err != nil {
		return fmt.Errorf("checking if remote %q exists: %v", defaultRemote, err)
	}

	var defRemote *git.Remote

	gitcfg := p.GitConfig()

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

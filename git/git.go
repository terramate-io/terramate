// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type (
	// Config configures the wrapper.
	Config struct {
		Username    string // Username used in commits.
		Email       string // Email used in commits.
		ProgramPath string

		// WorkingDir sets the directory where the commands will be applied.
		WorkingDir string

		// Env is the environment variables to be passed over to git.
		// If it is nil it means no environment variables should be passed.
		// To inherit all env vars from the parent process os.Getenv() needs
		// to be passed explicitly.
		Env []string

		// Isolated tells if the wrapper should run with isolated
		// configurations, which means setting it to true will make the wrapper
		// not rely on the global/system configuration. It's useful for
		// reproducibility of scripts.
		Isolated bool

		// AllowPorcelain tells if the wrapper is allowed to execute porcelain
		// commands. It's useful if the user is not sure if all commands used by
		// their program are plumbing.
		AllowPorcelain bool

		// Global arguments that are automatically added when executing git commands.
		// This is useful for setting config overrides or other common flags.
		GlobalArgs []string
	}

	// Git is the wrapper object.
	Git struct {
		options Options
	}

	// Ref is a git reference.
	Ref struct {
		Name     string
		CommitID string
	}

	// Remote is a git remote.
	Remote struct {
		// Name of the remote reference
		Name string
		// Branches are all the branches the remote reference has
		Branches []string
	}

	// LogLine is a log summary.
	LogLine struct {
		CommitID string
		Message  string
	}

	// Error is the sentinel error type.
	Error string

	// CmdError is the error for failed commands.
	CmdError struct {
		cmd    string // Command-line executed
		stdout []byte // stdout of the failed command
		stderr []byte // stderr of the failed command
	}

	// CommitMetadata is metadata associated with a Git commit.
	CommitMetadata struct {
		Author  string
		Email   string
		Time    *time.Time
		Subject string
		Body    string
	}
)

const (
	// ErrGitNotFound is the error that tells if git was not found.
	ErrGitNotFound Error = "git program not found"

	// ErrInvalidConfig is the error that tells if the configuration is invalid.
	ErrInvalidConfig Error = "invalid configuration"

	// ErrDenyPorcelain is the error that tells if a porcelain method was called
	// when AllowPorcelain is false.
	ErrDenyPorcelain Error = "porcelain commands are not allowed by the configuration"
)

type remoteSorter []Remote

// WithConfig creates a new git wrapper by providing the config.
func WithConfig(cfg Config) (*Git, error) {
	git := &Git{}
	git.options.config = cfg

	err := git.applyDefaults()
	if err != nil {
		return nil, fmt.Errorf("applying default config values: %w", err)
	}

	err = git.validate()
	if err != nil {
		return nil, err
	}

	_, err = git.Version()
	if err != nil {
		return nil, err
	}
	return git, nil
}

// With returns a copy of the wrapper options.
// Use opt.Wrapper() to get a new [Git] wrapper with the new options applied.
func (git *Git) With() *Options {
	copied := git.options
	copy(copied.config.Env, git.options.config.Env)
	copy(copied.config.GlobalArgs, git.options.config.GlobalArgs)
	return &copied
}

func (git *Git) applyDefaults() error {
	cfg := git.cfg()
	if cfg.ProgramPath == "" {
		programPath, err := exec.LookPath("git")
		if err != nil {
			return fmt.Errorf("%w: %v", ErrGitNotFound, err)
		}

		cfg.ProgramPath = programPath
	}

	if cfg.WorkingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		cfg.WorkingDir = wd
	}
	return nil
}

func (git *Git) validate() error {
	cfg := git.cfg()
	_, err := os.Stat(cfg.ProgramPath)
	if err != nil {
		return fmt.Errorf("failed to stat git program path \"%s\": %w: %v",
			cfg.ProgramPath, ErrInvalidConfig, err)
	}

	// DefaultBranch and DefaultRemote cannot be validated yet because the
	// repository needs to be initialized and git wrapper can be used to
	// initialize a repository with any branch and remote names user wants.

	return nil
}

// Version of the git program.
func (git *Git) Version() (string, error) {
	cfg := git.cfg()
	logger := log.With().
		Str("action", "Version()").
		Str("workingDir", cfg.WorkingDir).
		Logger()

	logger.Debug().Msg("Get git version.")
	out, err := git.exec("version")
	if err != nil {
		return "", err
	}

	const expected = "git version "

	// git version 2.33.1
	if strings.HasPrefix(out, expected) {
		return out[len(expected):], nil
	}
	return "", fmt.Errorf("unexpected \"git version\" output: %q", out)
}

// Init initializes a git repository. If bare is true, it initializes a "bare
// repository", in other words, a repository not intended for work but just
// store revisions.
// Beware: Init is a porcelain method.
func (git *Git) Init(dir string, defaultBranch string, bare bool) error {
	cfg := git.cfg()
	if !cfg.AllowPorcelain {
		return fmt.Errorf("Init: %w", ErrDenyPorcelain)
	}

	args := []string{
		"-b", defaultBranch,
		"--template=",
	}

	if bare {
		args = append(args, "--bare")
	}

	args = append(args, dir)
	_, err := git.exec("init", args...)
	if err != nil {
		return err
	}

	// TODO(i4k): code below is not thread-safe

	bkwd := cfg.WorkingDir

	defer func() {
		cfg.WorkingDir = bkwd
	}()

	cfg.WorkingDir = dir

	if cfg.Username != "" {
		_, err = git.exec("config", "--local", "user.name", cfg.Username)
		if err != nil {
			return err
		}
	}

	if cfg.Email != "" {
		_, err = git.exec("config", "--local", "user.email", cfg.Email)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoteAdd adds a new remote name.
func (git *Git) RemoteAdd(name string, url string) error {
	_, err := git.exec("remote", "add", name, url)
	return err
}

// HasRemotes returns if a there are any remotes configured.
func (git *Git) HasRemotes() (bool, error) {
	res, err := git.exec("config", "--get-regexp", "remote\\.")
	if err != nil {
		return false, err
	}

	return res != "", nil
}

// Remotes returns a list of all configured remotes and their respective branches.
// The result slice is ordered lexicographically by the remote name.
//
// Returns an empty list if no remote is found.
func (git *Git) Remotes() ([]Remote, error) {
	const refprefix = "refs/remotes/"

	res, err := git.exec("for-each-ref", "--format", "%(refname)", refprefix)

	if err != nil {
		return nil, err
	}

	if res == "" {
		return nil, nil
	}

	references := map[string][]string{}

	for _, rawref := range strings.Split(res, "\n") {
		trimmedref := strings.TrimPrefix(rawref, refprefix)
		parsed := strings.Split(trimmedref, "/")
		if len(parsed) < 2 {
			return nil, fmt.Errorf("unexpected remote reference %q", rawref)
		}
		name := parsed[0]
		branch := strings.Join(parsed[1:], "/")
		branches := references[name]
		references[name] = append(branches, branch)
	}

	var remotes remoteSorter

	for name, branches := range references {
		remotes = append(remotes, Remote{Name: name, Branches: branches})
	}

	sort.Stable(remotes)
	return remotes, nil
}

// LogSummary returns a list of commit log summary in reverse chronological
// order from the revs set operation. It expects the same revision list as the
// `git rev-list` command.
//
// It returns only the first line of the commit message.
func (git *Git) LogSummary(revs ...string) ([]LogLine, error) {
	if len(revs) == 0 {
		revs = append(revs, "HEAD")
	}

	args := append([]string{}, "--pretty=oneline")
	args = append(args, revs...)

	out, err := git.exec("rev-list", args...)
	if err != nil {
		return nil, err
	}

	logs := []LogLine{}

	lines := strings.Split(out, "\n")

	for _, line := range lines {
		l := strings.TrimSpace(line)
		if len(l) == 0 {
			break
		}

		index := strings.Index(l, " ")
		if index == -1 {
			return nil, fmt.Errorf("malformed log line")
		}

		logs = append(logs, LogLine{
			CommitID: l[0:index],
			Message:  l[index+1:],
		})
	}

	return logs, nil
}

// Add files to current staged index.
// Beware: Add is a porcelain method.
func (git *Git) Add(files ...string) error {
	cfg := git.cfg()
	if !cfg.AllowPorcelain {
		return fmt.Errorf("Add: %w", ErrDenyPorcelain)
	}

	log.Debug().
		Str("action", "Add()").
		Str("workingDir", cfg.WorkingDir).
		Msg("Add file to current staged index.")
	_, err := git.exec("add", files...)
	return err
}

// Clone will clone the given repo inside the given dir.
// Beware: Clone is a porcelain method.
func (git *Git) Clone(repoURL, dir string) error {
	cfg := git.cfg()
	if !cfg.AllowPorcelain {
		return fmt.Errorf("Clone: %w", ErrDenyPorcelain)
	}
	_, err := git.exec("clone", repoURL, dir)
	return err
}

// Commit the current staged changes.
// The args are extra flags and/or arguments to git commit command line.
// Beware: Commit is a porcelain method.
func (git *Git) Commit(msg string, args ...string) error {
	cfg := git.cfg()

	if !cfg.AllowPorcelain {
		return fmt.Errorf("Commit: %w", ErrDenyPorcelain)
	}

	for _, arg := range args {
		if arg == "-m" {
			return fmt.Errorf("the -m argument is already implicitly set")
		}
	}

	vargs := []string{
		"-m", msg,
	}

	vargs = append(vargs, args...)

	_, err := git.exec("commit", vargs...)
	return err
}

// RevParse parses the rev name and returns the commit id it points to.
// The rev name follows the [git revisions](https://git-scm.com/docs/gitrevisions)
// documentation.
func (git *Git) RevParse(rev string) (string, error) {
	return git.exec("rev-parse", rev)
}

// FetchRemoteRev will fetch from the remote repo the commit id and ref name
// for the given remote and reference. This will make use of the network
// to fetch data from the remote configured on the git repo.
func (git *Git) FetchRemoteRev(remote, ref string) (Ref, error) {
	cfg := git.cfg()
	logger := log.With().
		Str("action", "FetchRemoteRev()").
		Str("workingDir", cfg.WorkingDir).
		Logger()

	logger.Debug().Msg("List references in remote repository.")
	output, err := git.exec("ls-remote", remote, ref)
	if err != nil {
		return Ref{}, fmt.Errorf(
			"Git.FetchRemoteRev: git ls-remote %q %q: %v",
			remote,
			ref,
			err,
		)
	}
	parsed := strings.Split(output, "\t")
	if len(parsed) != 2 {
		return Ref{}, fmt.Errorf(
			"Git.FetchRemoteRev: git ls-remote %q %q can't parse: %v",
			remote,
			ref,
			output,
		)
	}
	return Ref{
		CommitID: parsed[0],
		Name:     parsed[1],
	}, nil
}

// MergeBase finds the common commit ancestor of commit1 and commit2.
func (git *Git) MergeBase(commit1, commit2 string) (string, error) {
	return git.exec("merge-base", commit1, commit2)
}

// Status returns the git status of the current branch.
// Beware: Status is a porcelain method.
func (git *Git) Status() (string, error) {
	if !git.cfg().AllowPorcelain {
		return "", fmt.Errorf("Status: %w", ErrDenyPorcelain)
	}

	return git.exec("status")
}

// DiffTree compares the from and to commit ids and returns the differences. If
// nameOnly is set then only the file names of changed files are show. If
// recurse is set, then it walks into child trees as well. If
// relative is set, then only show local changes of current dir.
func (git *Git) DiffTree(from, to string, relative, nameOnly, recurse bool) (string, error) {
	args := []string{from, to}

	if relative {
		args = append(args, "--relative")
	}

	if nameOnly {
		args = append(args, "--name-only")
	}

	if recurse {
		args = append(args, "-r") // git help shows no long flag name
	}

	return git.exec("diff-tree", args...)
}

// DiffNames recursively walks the git tree objects computing the from and to
// commit ids differences and return all the file names containing differences
// relative to configuration WorkingDir.
func (git *Git) DiffNames(from, to string) ([]string, error) {
	diff, err := git.DiffTree(from, to, true, true, true)
	if err != nil {
		return nil, fmt.Errorf("diff-tree: %w", err)
	}

	return removeEmptyLines(strings.Split(diff, "\n")), nil
}

// NewBranch creates a new branch reference pointing to current HEAD.
func (git *Git) NewBranch(name string) error {
	_, err := git.RevParse(name)
	if err == nil {
		return fmt.Errorf("branch \"%s\" already exists", name)
	}

	log.Debug().
		Str("action", "NewBranch()").
		Str("workingDir", git.cfg().WorkingDir).
		Str("reference", name).
		Msg("Create new branch.")
	_, err = git.exec("update-ref", "refs/heads/"+name, "HEAD")
	return err
}

// DeleteBranch deletes the branch.
func (git *Git) DeleteBranch(name string) error {
	_, err := git.RevParse(name)
	if err != nil {
		return fmt.Errorf("branch \"%s\" doesn't exist", name)
	}

	log.Debug().
		Str("action", "DeleteBranch()").
		Str("workingDir", git.cfg().WorkingDir).
		Str("reference", name).
		Msg("Delete branch.")
	_, err = git.exec("update-ref", "-d", "refs/heads/"+name)
	return err
}

// Checkout switches branches or change to specific revisions in the tree.
// When switching branches, the create flag can be set to automatically create
// the new branch before changing into it.
// Beware: Checkout is a porcelain method.
func (git *Git) Checkout(rev string, create bool) error {
	if !git.cfg().AllowPorcelain {
		return fmt.Errorf("checkout: %w", ErrDenyPorcelain)
	}

	if create {
		err := git.NewBranch(rev)
		if err != nil {
			return err
		}
	}

	log.Debug().
		Str("action", "Checkout()").
		Str("workingDir", git.cfg().WorkingDir).
		Str("reference", rev).
		Msg("Checkout.")
	_, err := git.exec("checkout", rev)
	return err
}

// Merge branch into current branch using the non fast-forward strategy.
// Beware: Merge is a porcelain method.
func (git *Git) Merge(branch string) error {
	if !git.cfg().AllowPorcelain {
		return fmt.Errorf("Merge: %w", ErrDenyPorcelain)
	}

	log.Debug().
		Str("action", "Merge()").
		Str("workingDir", git.cfg().WorkingDir).
		Str("reference", branch).
		Msg("Merge.")
	_, err := git.exec("merge", "--no-ff", branch)
	return err
}

// Push changes from branch onto remote.
func (git *Git) Push(remote, branch string) error {
	if !git.cfg().AllowPorcelain {
		return fmt.Errorf("Push: %w", ErrDenyPorcelain)
	}
	_, err := git.exec("push", remote, branch)
	return err
}

// Pull changes from remote into branch
func (git *Git) Pull(remote, branch string) error {
	if !git.cfg().AllowPorcelain {
		return fmt.Errorf("Pull: %w", ErrDenyPorcelain)
	}
	_, err := git.exec("pull", remote, branch)
	return err
}

// ListUntracked lists untracked files in the directories provided in dirs.
func (git *Git) ListUntracked(dirs ...string) ([]string, error) {
	args := []string{
		"--others", "--exclude-standard",
	}

	if len(dirs) > 0 {
		args = append(args, "--")
		args = append(args, dirs...)
	}

	log.Debug().
		Str("action", "ListUntracked()").
		Str("workingDir", git.cfg().WorkingDir).
		Msg("List untracked files.")
	out, err := git.exec("ls-files", args...)
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}

	return removeEmptyLines(strings.Split(out, "\n")), nil

}

// ListUncommitted lists uncommitted files in the directories provided in dirs.
func (git *Git) ListUncommitted(dirs ...string) ([]string, error) {
	args := []string{
		"--modified", "--exclude-standard",
	}

	if len(dirs) > 0 {
		args = append(args, "--")
		args = append(args, dirs...)
	}

	log.Debug().
		Str("action", "ListUncommitted()").
		Str("workingDir", git.cfg().WorkingDir).
		Msg("List uncommitted files.")
	out, err := git.exec("ls-files", args...)
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}

	return removeEmptyLines(strings.Split(out, "\n")), nil
}

// ShowCommitMetadata returns common metadata associated with the given object.
// An object name can be a commit SHA or a symbolic name, i.e. HEAD, branch-name, etc.
func (git *Git) ShowCommitMetadata(objectName string) (*CommitMetadata, error) {
	// %n - newline
	// %an - author name
	// %at - author time (unix)
	// %ae - author email
	// %s - commit msg subject
	// %b - commit msg body
	out, err := git.exec("show", "-s", "--format=%an%n%at%n%ae%n%s%n%b", objectName)
	if err != nil {
		return nil, fmt.Errorf("show: %w", err)
	}

	lines := strings.SplitN(out, "\n", 5)
	// 4 lines without body, 5+ with body
	if len(lines) < 4 {
		return nil, fmt.Errorf("show metadata: malformed output: %v", out)
	}

	author := strings.TrimSpace(lines[0])

	var commitTime *time.Time
	unixTime, err := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if err == nil {
		v := time.Unix(unixTime, 0)
		commitTime = &v
	} else {
		commitTime = nil
	}

	email := strings.TrimSpace(lines[2])
	subject := strings.TrimSpace(lines[3])

	body := ""
	if len(lines) > 4 {
		body = strings.TrimSpace(lines[4])
	}

	return &CommitMetadata{
		Author:  author,
		Time:    commitTime,
		Email:   email,
		Subject: subject,
		Body:    body,
	}, nil
}

// Root returns the git root directory.
func (git *Git) Root() (string, error) {
	return git.exec("rev-parse", "--show-toplevel")
}

// IsRepository tell if the git wrapper setup is operating in a valid git
// repository.
func (git *Git) IsRepository() bool {
	_, err := git.Root()
	return err == nil
}

// AddSubmodule adds the submodule name from url into this repository.
// For security reasons, this method should only be used in tests.
func (git *Git) AddSubmodule(name string, url string) (string, error) {
	if !git.cfg().AllowPorcelain {
		return "", fmt.Errorf("AddSubmodule: %w", ErrDenyPorcelain)
	}
	return git.exec("-c", "protocol.file.allow=always", "submodule", "add", url, name)
}

// Exec executes any provided git command. We don't allow Exec if AllowPorcelain
// is set to false.
func (git *Git) Exec(command string, args ...string) (string, error) {
	if !git.cfg().AllowPorcelain {
		return "", fmt.Errorf("Exec: %w", ErrDenyPorcelain)
	}
	return git.exec(command, args...)
}

// CurrentBranch returns the short branch name that HEAD points to.
func (git *Git) CurrentBranch() (string, error) {
	return git.exec("symbolic-ref", "--short", "HEAD")
}

// SetRemoteURL sets the remote url.
func (git *Git) SetRemoteURL(remote, url string) error {
	if !git.cfg().AllowPorcelain {
		return fmt.Errorf("SetRemoteURL: %w", ErrDenyPorcelain)
	}
	_, err := git.exec("remote", "set-url", remote, url)
	return err
}

// URL returns the remote URL.
func (git *Git) URL(remote string) (string, error) {
	return git.exec("remote", "get-url", remote)
}

// GetConfigValue returns the value mapped to given config key, or an error if they doesn't exist.
func (git *Git) GetConfigValue(key string) (string, error) {
	s, err := git.exec("config", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (git *Git) exec(command string, args ...string) (string, error) {
	cfg := git.cfg()
	cmd := exec.Cmd{
		Path: cfg.ProgramPath,
		Args: []string{cfg.ProgramPath},
		Dir:  cfg.WorkingDir,
		Env:  []string{},
	}

	cmd.Args = append(cmd.Args, cfg.GlobalArgs...)
	cmd.Args = append(cmd.Args, command)
	cmd.Args = append(cmd.Args, args...)

	// nil and empty slice behave differently on exec.Cmd.
	// nil defaults to use parent env, empty means actually empty.
	// we want nil and empty to behave the same (no env).
	if cfg.Env != nil {
		cmd.Env = cfg.Env
	}

	if cfg.Isolated {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_SYSTEM=/dev/null")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOGLOBAL=1") // back-compat
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1") // back-compat
		cmd.Env = append(cmd.Env, "GIT_ATTR_NOSYSTEM=1")
	}

	stdout, err := cmd.Output()
	if err != nil {
		stderr := []byte{}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr = exitError.Stderr
		}
		return "", NewCmdError(cmd.String(), stdout, stderr)
	}
	out := strings.TrimSpace(string(stdout))
	return out, nil
}

func (git *Git) cfg() *Config { return &git.options.config }

// Error string representation.
func (e Error) Error() string {
	return string(e)
}

// NewCmdError returns a new command line error.
func NewCmdError(cmd string, stdout, stderr []byte) error {
	return &CmdError{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
	}
}

// Is tells if err is of the type CmdError.
func (e *CmdError) Is(err error) bool {
	_, ok := err.(*CmdError)
	return ok
}

// Error string representation.
func (e *CmdError) Error() string {
	return fmt.Sprintf("failed to exec: %s : stderr=%q, stdout=%q",
		e.cmd, string(e.stderr), string(e.stdout))
}

// ShortCommitID returns the short version of the commit ID.
// If the reference doesn't have a valid commit id it returns empty.
func (r Ref) ShortCommitID() string {
	if len(r.CommitID) < 8 {
		return ""
	}
	return r.CommitID[0:8]
}

// Command is the failed command.
func (e *CmdError) Command() string { return e.cmd }

// Stdout of the failed command.
func (e *CmdError) Stdout() []byte { return e.stdout }

// Stderr of the failed command.
func (e *CmdError) Stderr() []byte { return e.stderr }

func (r remoteSorter) Len() int {
	return len(r)
}

func (r remoteSorter) Less(i, j int) bool {
	return r[i].Name < r[j].Name
}

func (r remoteSorter) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func removeEmptyLines(lines []string) []string {
	outlines := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			outlines = append(outlines, line)
		}
	}
	return outlines
}

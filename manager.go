// Copyright 2021 Mineiros GmbH
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

package terramate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

type (
	// Manager is the terramate stacks manager.
	Manager struct {
		root       string // root is the project's root directory
		gitBaseRef string // gitBaseRef is the git ref where we compare changes.

		stackLoader stack.Loader
	}

	// StacksReport is the report of project's stacks and the result of its
	// default checks.
	StacksReport struct {
		Stacks []Entry

		// Checks contains the result info of default checks.
		Checks RepoChecks
	}

	// RepoChecks contains the info of default checks.
	RepoChecks struct {
		UncommittedFiles []string
		UntrackedFiles   []string
	}

	// Entry is a stack entry result.
	Entry struct {
		Stack  stack.S
		Reason string // Reason why this entry was returned.
	}
)

// NewManager creates a new stack manager. The rootdir is the project's
// directory and gitBaseRef is the git reference to compare against for changes.
func NewManager(rootdir string, gitBaseRef string) *Manager {
	return &Manager{
		root:        rootdir,
		gitBaseRef:  gitBaseRef,
		stackLoader: stack.NewLoader(rootdir),
	}
}

// List walks the basedir directory looking for terraform stacks.
// It returns a lexicographic sorted list of stack directories.
func (m *Manager) List() (*StacksReport, error) {
	logger := log.With().
		Str("action", "Manager.List()").
		Logger()

	logger.Debug().Msg("List stacks.")

	entries, err := ListStacks(m.root)
	if err != nil {
		return nil, err
	}

	report := &StacksReport{
		Stacks: entries,
	}

	logger.Trace().Str("repo", m.root).Msg("Create git wrapper for repo.")

	g, err := git.WithConfig(git.Config{
		WorkingDir: m.root,
	})
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Check if path is git repo.")
	if !g.IsRepository() {
		return report, nil
	}

	report.Checks, err = checkRepoIsClean(g)
	if err != nil {
		return nil, err
	}

	return report, nil
}

// ListChanged lists the stacks that have changed on the current branch,
// compared to the main branch. This method assumes a version control
// system in place and that you are working on a branch that is not main.
// It's an error to call this method in a directory that's not
// inside a repository or a repository with no commits in it.
func (m *Manager) ListChanged() (*StacksReport, error) {
	logger := log.With().
		Str("action", "ListChanged()").
		Logger()

	logger.Trace().Msg("Create git wrapper on project root.")

	g, err := git.WithConfig(git.Config{
		WorkingDir: m.root,
	})

	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Check if path is git repo.")

	if !g.IsRepository() {
		return nil, fmt.Errorf("the path \"%s\" is not a git repository", m.root)
	}

	checks, err := checkRepoIsClean(g)
	if err != nil {
		return nil, err
	}

	logger.Debug().Msg("List changed files.")

	files, err := listChangedFiles(m.root, m.gitBaseRef)
	if err != nil {
		return nil, err
	}

	stackSet := map[string]Entry{}

	logger.Trace().
		Msg("Range over files.")
	for _, path := range files {
		logger.Trace().
			Msg("Get dir name.")
		dirname := filepath.Dir(filepath.Join(m.root, path))

		if _, ok := stackSet[project.PrjAbsPath(m.root, dirname)]; ok {
			continue
		}

		logger.Debug().
			Str("path", dirname).
			Msg("Try load changed.")
		s, found, err := m.stackLoader.TryLoadChanged(m.root, dirname)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.E("listing changed files", err)
		}

		if !found {
			logger.Debug().
				Str("path", dirname).
				Msg("Lookup parent stack.")
			s, found, err = stack.LookupParent(m.root, dirname)
			if err != nil {
				return nil, errors.E("listing changed files", err)
			}

			if !found {
				continue
			}
		}

		stackSet[s.PrjAbsPath()] = Entry{
			Stack:  s,
			Reason: "stack has unmerged changes",
		}
	}

	logger.Debug().Msg("Get list of all stacks.")

	allstacks, err := ListStacks(m.root)
	if err != nil {
		return nil, errors.E("searching for stacks", err)
	}

	logger.Trace().Msg("Range over all stacks.")

	for _, stackEntry := range allstacks {
		stack := stackEntry.Stack
		if _, ok := stackSet[stack.PrjAbsPath()]; ok {
			continue
		}

		logger.Debug().
			Stringer("stack", stack).
			Msg("Apply function to stack.")

		err := m.filesApply(stack.AbsPath(), func(file fs.DirEntry) error {
			if path.Ext(file.Name()) != ".tf" {
				return nil
			}

			logger.Debug().
				Stringer("stack", stack).
				Msg("Get tf file path.")

			tfpath := filepath.Join(stack.AbsPath(), file.Name())

			logger.Trace().
				Stringer("stack", stack).
				Str("configFile", tfpath).
				Msg("Parse modules.")

			modules, err := hcl.ParseModules(tfpath)
			if err != nil {
				return errors.E("parsing modules", err)
			}

			logger.Trace().
				Stringer("stack", stack).
				Str("configFile", tfpath).
				Msg("Range over modules.")

			for _, mod := range modules {
				logger.Trace().
					Stringer("stack", stack).
					Str("configFile", tfpath).
					Msg("Check if module changed.")

				changed, why, err := m.moduleChanged(mod, stack.AbsPath(), make(map[string]bool))
				if err != nil {
					return fmt.Errorf("checking module %q: %w", mod.Source, err)
				}

				if changed {
					logger.Debug().
						Stringer("stack", stack).
						Str("configFile", tfpath).
						Msg("Module changed.")

					stack.SetChanged(true)
					stackSet[stack.PrjAbsPath()] = Entry{
						Stack:  stack,
						Reason: fmt.Sprintf("stack changed because %q changed because %s", mod.Source, why),
					}
					return nil
				}
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("checking module changes: %w", err)
		}
	}

	logger.Trace().Msg("Make set of changed stacks.")

	changedStacks := make([]Entry, 0, len(stackSet))
	for _, stack := range stackSet {
		changedStacks = append(changedStacks, stack)
	}

	logger.Trace().Msg("Sort changed stacks.")

	sort.Sort(EntrySlice(changedStacks))

	return &StacksReport{
		Checks: checks,
		Stacks: changedStacks,
	}, nil
}

func (m *Manager) filesApply(dir string, apply func(file fs.DirEntry) error) error {
	logger := log.With().
		Str("action", "filesApply()").
		Str("path", dir).
		Logger()

	logger.Debug().
		Msg("Read dir.")
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("listing files of directory %q: %w", dir, err)
	}

	logger.Trace().
		Msg("Range files in dir.")
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		logger.Debug().
			Msg("Apply function to file.")
		err := apply(file)
		if err != nil {
			return fmt.Errorf("applying operation to file %q: %w", file, err)
		}
	}

	return nil
}

// moduleChanged recursively check if the module mod or any of the modules it
// uses has changed. All .tf files of the module are parsed and this function is
// called recursively. The visited keep track of the modules already parsed to
// avoid infinite loops.
func (m *Manager) moduleChanged(
	mod hcl.Module, basedir string, visited map[string]bool,
) (changed bool, why string, err error) {
	logger := log.With().
		Str("action", "moduleChanged()").
		Logger()

	if _, ok := visited[mod.Source]; ok {
		return false, "", nil
	}

	logger.Trace().
		Str("path", basedir).
		Msg("Check if module source is local directory.")
	if !mod.IsLocal() {
		// if the source is a remote path (URL, VCS path, S3 bucket, etc) then
		// we assume it's not changed.

		return false, "", nil
	}

	logger.Trace().
		Str("path", basedir).
		Msg("Get module path.")
	modPath := filepath.Join(basedir, mod.Source)

	logger.Trace().
		Str("path", modPath).
		Msg("Get module path info.")
	st, err := os.Stat(modPath)

	// TODO(i4k): resolve symlinks

	if err != nil || !st.IsDir() {
		return false, "", fmt.Errorf("\"source\" path %q is not a directory", modPath)
	}

	logger.Debug().
		Str("path", modPath).
		Msg("Get list of changed files.")
	changedFiles, err := listChangedFiles(modPath, m.gitBaseRef)
	if err != nil {
		return false, "", fmt.Errorf("listing changes in the module %q: %w",
			mod.Source, err)
	}

	if len(changedFiles) > 0 {
		return true, fmt.Sprintf("module %q has unmerged changes", mod.Source), nil
	}

	visited[mod.Source] = true

	logger.Debug().
		Str("path", modPath).
		Msg("Apply function to files in path.")
	err = m.filesApply(modPath, func(file fs.DirEntry) error {
		if changed {
			return nil
		}
		if path.Ext(file.Name()) != ".tf" {
			return nil
		}

		logger.Trace().
			Str("path", modPath).
			Msg("Parse modules.")
		modules, err := hcl.ParseModules(filepath.Join(modPath, file.Name()))
		if err != nil {
			return fmt.Errorf("parsing module %q: %w", mod.Source, err)
		}

		logger.Trace().
			Str("path", modPath).
			Msg("Range over modules.")
		for _, mod2 := range modules {
			var reason string

			logger.Trace().
				Str("path", modPath).
				Msg("Get if module is changed.")
			changed, reason, err = m.moduleChanged(mod2, modPath, visited)
			if err != nil {
				return err
			}

			if changed {
				logger.Trace().
					Str("path", modPath).
					Msg("Module was changed.")
				why = fmt.Sprintf("%s%s changed because %s ", why, mod.Source,
					reason)
				return nil
			}
		}

		return nil
	})

	if err != nil {
		return false, "", err
	}

	return changed, fmt.Sprintf("module %q changed because %s", mod.Source, why), nil
}

// AddWantedOf returns all wanted stacks from the given stacks.
func (m *Manager) AddWantedOf(stacks []stack.S) ([]stack.S, error) {
	wantedBy := map[string]stack.S{}
	wanted := []stack.S{}

	for _, s := range stacks {
		wantedBy[s.PrjAbsPath()] = s
		wanted = append(wanted, s)
	}

	visited := map[string]struct{}{}

	for len(wantedBy) > 0 {
		for _, s := range wantedBy {
			logger := log.With().
				Str("action", "AddWantedOf()").
				Stringer("stack", s).
				Logger()

			logger.Debug().Msg("Loading \"wanted\" stacks.")

			wantedStacks, err := m.stackLoader.LoadAll(m.root, s.AbsPath(), s.Wants()...)
			if err != nil {
				return nil, fmt.Errorf("calculating wanted stacks: %v", err)
			}

			logger.Debug().Msg("The \"wanted\" stacks were loaded successfully.")

			for _, wantedStack := range wantedStacks {
				if wantedStack.AbsPath() == s.AbsPath() {
					logger.Warn().
						Stringer("stack", s).
						Msg("stack wants itself.")

					continue
				}

				if _, ok := visited[wantedStack.PrjAbsPath()]; !ok {
					wanted = append(wanted, wantedStack)
					visited[wantedStack.PrjAbsPath()] = struct{}{}
					wantedBy[wantedStack.PrjAbsPath()] = wantedStack
				}
			}

			delete(wantedBy, s.PrjAbsPath())
		}
	}

	stack.Sort(wanted)
	return wanted, nil
}

// listChangedFiles lists all changed files in the dir directory.
func listChangedFiles(dir string, gitBaseRef string) ([]string, error) {
	logger := log.With().
		Str("action", "listChangedFiles()").
		Str("path", dir).
		Logger()

	logger.Trace().Msg("Get dir info.")

	st, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	logger.Trace().Msg("Check if path is dir.")

	if !st.IsDir() {
		return nil, fmt.Errorf("is not a directory")
	}

	logger.Trace().Msg("Create git wrapper with dir.")

	g, err := git.WithConfig(git.Config{
		WorkingDir: dir,
	})
	if err != nil {
		return nil, err
	}

	logger.Trace().
		Msg("Get commit id of git base ref.")
	baseRef, err := g.RevParse(gitBaseRef)
	if err != nil {
		return nil, fmt.Errorf("getting revision %q: %w", gitBaseRef, err)
	}

	logger.Trace().
		Msg("Get commit id of HEAD.")
	headRef, err := g.RevParse("HEAD")
	if err != nil {
		return nil, fmt.Errorf("getting HEAD revision: %w", err)
	}

	if baseRef == headRef {
		return []string{}, nil
	}

	logger.Trace().
		Msg("Find common commit ancestor of HEAd and base ref.")
	mergeBaseRef, err := g.MergeBase("HEAD", baseRef)
	if err != nil {
		return nil, fmt.Errorf("getting merge-base HEAD main: %w", err)
	}

	if baseRef != mergeBaseRef {
		return nil, fmt.Errorf("main branch is not reachable: main ref %q can't reach %q",
			baseRef, mergeBaseRef)
	}

	return g.DiffNames(baseRef, headRef)
}

func checkRepoIsClean(g *git.Git) (RepoChecks, error) {
	logger := log.With().
		Str("action", "checkRepoIsClean()").
		Logger()

	logger.Debug().Msg("Get list of untracked files.")

	untracked, err := g.ListUntracked()
	if err != nil {
		return RepoChecks{}, fmt.Errorf("listing untracked files: %v", err)
	}

	logger.Debug().Msg("Get list of uncommitted files in dir.")

	uncommitted, err := g.ListUncommitted()
	if err != nil {
		return RepoChecks{}, fmt.Errorf("listing uncommitted files: %v", err)
	}

	return RepoChecks{
		UntrackedFiles:   untracked,
		UncommittedFiles: uncommitted,
	}, nil
}

// EntrySlice implements the Sort interface.
type EntrySlice []Entry

func (x EntrySlice) Len() int           { return len(x) }
func (x EntrySlice) Less(i, j int) bool { return x[i].Stack.PrjAbsPath() < x[j].Stack.PrjAbsPath() }
func (x EntrySlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

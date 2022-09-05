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
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/run"
	"github.com/mineiros-io/terramate/run/dag"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/tf"
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
		Stack  *stack.S
		Reason string // Reason why this entry was returned.
	}
)

const errList errors.Kind = "listing stacks error"
const errListChanged errors.Kind = "listing changed stacks error"

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
		return nil, errors.E(errList, err)
	}

	logger.Trace().Msg("Check if path is git repo.")
	if !g.IsRepository() {
		return report, nil
	}

	report.Checks, err = checkRepoIsClean(g)
	if err != nil {
		return nil, errors.E(errList, err)
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
		return nil, errors.E(errListChanged, err)
	}

	logger.Trace().Msg("Check if path is git repo.")

	if !g.IsRepository() {
		return nil, errors.E(errListChanged, "the path \"%s\" is not a git repository", m.root)
	}

	checks, err := checkRepoIsClean(g)
	if err != nil {
		return nil, errors.E(errListChanged, err)
	}

	logger.Debug().Msg("List changed files.")

	changedFiles, err := listChangedFiles(m.root, m.gitBaseRef)
	if err != nil {
		return nil, errors.E(errListChanged, err)
	}

	stackSet := map[string]Entry{}

	logger.Trace().
		Msg("Range over files.")
	for _, path := range changedFiles {
		if strings.HasPrefix(path, ".") {
			continue
		}

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
			return nil, errors.E(errListChanged, err)
		}

		if !found {
			logger.Debug().
				Str("path", dirname).
				Msg("Lookup parent stack.")
			s, found, err = stack.LookupParent(m.root, dirname)
			if err != nil {
				return nil, errors.E(errListChanged, err)
			}

			if !found {
				continue
			}
		}

		stackSet[s.Path()] = Entry{
			Stack:  s,
			Reason: "stack has unmerged changes",
		}
	}

	logger.Debug().Msg("Get list of all stacks.")

	allstacks, err := ListStacks(m.root)
	if err != nil {
		return nil, errors.E(errListChanged, "searching for stacks", err)
	}

	logger.Trace().Msg("Range over all stacks.")

rangeStacks:
	for _, stackEntry := range allstacks {
		stack := stackEntry.Stack
		if _, ok := stackSet[stack.Path()]; ok {
			continue
		}

		logger.Debug().
			Stringer("stack", stack).
			Msg("Check for changed watch files.")

		if changed, ok := hasChangedWatchedFiles(stack, changedFiles); ok {
			logger.Debug().
				Stringer("stack", stack).
				Str("watchfile", changed).
				Msg("changed.")

			stack.SetChanged(true)
			stackSet[stack.Path()] = Entry{
				Stack: stack,
				Reason: fmt.Sprintf(
					"stack changed because watched file %q changed",
					changed,
				),
			}
			continue rangeStacks
		}

		logger.Debug().
			Stringer("stack", stack).
			Msg("Apply function to stack.")

		err := m.filesApply(stack.HostPath(), func(file fs.DirEntry) error {
			if path.Ext(file.Name()) != ".tf" {
				return nil
			}

			logger.Debug().
				Stringer("stack", stack).
				Msg("Get tf file path.")

			tfpath := filepath.Join(stack.HostPath(), file.Name())

			logger.Trace().
				Stringer("stack", stack).
				Str("configFile", tfpath).
				Msg("Parse modules.")

			modules, err := tf.ParseModules(tfpath)
			if err != nil {
				return errors.E(errListChanged, "parsing modules", err)
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

				changed, why, err := m.moduleChanged(mod, stack.HostPath(), make(map[string]bool))
				if err != nil {
					return errors.E(errListChanged, err, "checking module %q", mod.Source)
				}

				if changed {
					logger.Debug().
						Stringer("stack", stack).
						Str("configFile", tfpath).
						Msg("Module changed.")

					stack.SetChanged(true)
					stackSet[stack.Path()] = Entry{
						Stack: stack,
						Reason: fmt.Sprintf(
							"stack changed because %q changed because %s",
							mod.Source, why,
						),
					}
					return nil
				}
			}
			return nil
		})

		if err != nil {
			return nil, errors.E(errListChanged, "checking module changes", err)
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
		return errors.E(err, "listing files of directory %q", dir)
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
			return errors.E(err, "applying operation to file %q", file.Name())
		}
	}

	return nil
}

// moduleChanged recursively check if the module mod or any of the modules it
// uses has changed. All .tf files of the module are parsed and this function is
// called recursively. The visited keep track of the modules already parsed to
// avoid infinite loops.
func (m *Manager) moduleChanged(
	mod tf.Module, basedir string, visited map[string]bool,
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
		return false, "", errors.E("\"source\" path %q is not a directory", modPath)
	}

	logger.Debug().
		Str("path", modPath).
		Msg("Get list of changed files.")
	changedFiles, err := listChangedFiles(modPath, m.gitBaseRef)
	if err != nil {
		return false, "", errors.E(err,
			"listing changes in the module %q",
			mod.Source)
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
		modules, err := tf.ParseModules(filepath.Join(modPath, file.Name()))
		if err != nil {
			return errors.E(err, "parsing module %q", mod.Source)
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
				why = fmt.Sprintf("%s%s changed because %s ", why, mod.Source, reason)
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
func (m *Manager) AddWantedOf(scopeStacks stack.List) (stack.List, error) {
	logger := log.With().
		Str("action", "manager.AddWantedOf").
		Logger()

	wantsDag := dag.New()
	loader := stack.NewLoader(m.root)

	allstacks, err := stack.LoadAll(m.root)
	if err != nil {
		return nil, err
	}

	visited := run.Visited{}
	sort.Sort(allstacks)
	for _, s := range allstacks {
		loader.Set(s.Path(), s)

		logger.Trace().
			Str("stack", s.Path()).
			Msg("Building dag")

		err := run.BuildDAG(
			wantsDag,
			m.root,
			s,
			loader,
			stack.S.WantedBy,
			stack.S.Wants,
			visited,
		)

		if err != nil {
			return nil, err
		}
	}

	logger.Trace().Msg("Validating DAG.")

	reason, err := wantsDag.Validate()
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			logger.Warn().
				Err(err).
				Str("reason", reason).
				Msg(`Ignored cycle while validating the "wants" and "wanted_by" of stacks`)
		} else {
			logger.Warn().
				Err(err).
				Msg(`Ignored error while validating "wants" and "wanted_by`)
		}
	}

	var selectedStacks stack.List
	visited = run.Visited{}
	addStack := func(s *stack.S) {
		if _, ok := visited[s.Path()]; ok {
			return
		}

		visited[s.Path()] = struct{}{}
		selectedStacks = append(selectedStacks, s)
	}

	var pending []dag.ID
	for _, s := range scopeStacks {
		pending = append(pending, dag.ID(s.Path()))
	}

	for len(pending) > 0 {
		id := pending[0]
		node, _ := wantsDag.Node(id)
		s := node.(*stack.S)
		addStack(s)
		pending = pending[1:]

		ancestors := wantsDag.AncestorsOf(id)
		for _, id := range ancestors {
			if _, ok := visited[string(id)]; !ok {
				pending = append(pending, id)
			}
		}
	}
	return selectedStacks, nil
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
		return nil, errors.E(err, "stat failed on %q", dir)
	}

	logger.Trace().Msg("Check if path is dir.")

	if !st.IsDir() {
		return nil, errors.E("is not a directory")
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
		return nil, errors.E(err, "getting revision %q", gitBaseRef)
	}

	logger.Trace().
		Msg("Get commit id of HEAD.")
	headRef, err := g.RevParse("HEAD")
	if err != nil {
		return nil, errors.E(err, "getting HEAD revision")
	}

	if baseRef == headRef {
		return []string{}, nil
	}

	logger.Trace().
		Msg("Find common commit ancestor of HEAd and base ref.")
	mergeBaseRef, err := g.MergeBase("HEAD", baseRef)
	if err != nil {
		return nil, errors.E(err, "getting merge-base HEAD main")
	}

	if baseRef != mergeBaseRef {
		return nil, errors.E(
			"main branch is not reachable: main ref %q can't reach %q",
			baseRef, mergeBaseRef)
	}

	return g.DiffNames(baseRef, headRef)
}

func hasChangedWatchedFiles(stack *stack.S, changedFiles []string) (string, bool) {
	for _, watchFile := range stack.Watch() {
		for _, file := range changedFiles {
			if file == watchFile[1:] { // project paths
				return watchFile, true
			}
		}
	}
	return "", false
}

func checkRepoIsClean(g *git.Git) (RepoChecks, error) {
	logger := log.With().
		Str("action", "checkRepoIsClean()").
		Logger()

	logger.Debug().Msg("Get list of untracked files.")

	untracked, err := g.ListUntracked()
	if err != nil {
		return RepoChecks{}, errors.E(err, "listing untracked files")
	}

	logger.Debug().Msg("Get list of uncommitted files in dir.")

	uncommitted, err := g.ListUncommitted()
	if err != nil {
		return RepoChecks{}, errors.E(err, "listing uncommitted files")
	}

	return RepoChecks{
		UntrackedFiles:   untracked,
		UncommittedFiles: uncommitted,
	}, nil
}

// EntrySlice implements the Sort interface.
type EntrySlice []Entry

func (x EntrySlice) Len() int           { return len(x) }
func (x EntrySlice) Less(i, j int) bool { return x[i].Stack.Path() < x[j].Stack.Path() }
func (x EntrySlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

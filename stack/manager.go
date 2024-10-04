// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/stack/trigger"
	"github.com/terramate-io/terramate/tf"
	"github.com/terramate-io/terramate/tg"
)

type (
	// Manager is the terramate stacks manager.
	Manager struct {
		root *config.Root // whole config
		git  *git.Git

		cache struct {
			stacks       []Entry
			changedFiles []string
		}
	}

	// Report is the report of project's stacks and the result of its default checks.
	Report struct {
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
		Stack  *config.Stack
		Reason string // Reason why this entry was returned.
	}
)

const errList errors.Kind = "listing stacks error"
const errListChanged errors.Kind = "listing changed stacks error"

// NewManager creates a new stack manager.
func NewManager(root *config.Root) *Manager {
	return &Manager{
		root: root,
	}
}

// NewGitAwareManager returns a stack manager that supports change detection.
func NewGitAwareManager(root *config.Root, git *git.Git) *Manager {
	return &Manager{
		root: root,
		git:  git,
	}
}

// List walks the basedir directory looking for terraform stacks.
// The stacks are cached and sorted lexicographicly by the directory.
func (m *Manager) List(checkRepo bool) (*Report, error) {
	allstacks, err := m.allStacks()
	if err != nil {
		return nil, err
	}
	report := &Report{
		Stacks: allstacks,
	}

	if !checkRepo {
		return report, nil
	}

	if m.git == nil || !m.git.IsRepository() {
		return report, nil
	}

	report.Checks, err = checkRepoIsClean(m.git)
	if err != nil {
		return nil, errors.E(errList, err)
	}
	return report, nil
}

// ListChanged lists the stacks that have changed on the current HEAD,
// compared to the main branch. This method assumes a version control
// system in place and that you are working on a branch that is not main.
// It's an error to call this method in a directory that's not
// inside a repository or a repository with no commits in it.
// It never returns cached values.
func (m *Manager) ListChanged(gitBaseRef string) (*Report, error) {
	logger := log.With().
		Str("action", "ListChanged()").
		Logger()

	if !m.git.IsRepository() {
		return nil, errors.E(
			errListChanged,
			"the path \"%s\" is not a git repository",
			m.root.HostDir(),
		)
	}

	checks, err := checkRepoIsClean(m.git)
	if err != nil {
		return nil, errors.E(errListChanged, err)
	}

	if m.cache.changedFiles == nil {
		var err error

		m.cache.changedFiles, err = m.listChangedFiles(m.root.HostDir(), gitBaseRef)
		if err != nil {
			return nil, errors.E(errListChanged, err)
		}
	}

	stackSet := map[project.Path]Entry{}
	ignoreSet := map[project.Path]struct{}{}

	for _, path := range m.cache.changedFiles {
		abspath := filepath.Join(m.root.HostDir(), path)
		projpath := project.PrjAbsPath(m.root.HostDir(), abspath)

		logger = logger.With().
			Stringer("path", projpath).
			Logger()

		triggerInfo, triggeredStack, isTriggerFile, errTriggerParse := trigger.Is(m.root, projpath)
		if isTriggerFile {
			logger = logger.With().
				Stringer("trigger", triggeredStack).
				Logger()

			if _, err := os.Stat(abspath); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					logger.Debug().Msg("ignoring deleted trigger file")
					continue
				}
			}

			if errTriggerParse != nil {
				printer.Stderr.WarnWithDetails(
					fmt.Sprintf("skipping malformed trigger file: %s", projpath),
					errTriggerParse,
				)
				continue
			}

			logger.Debug().Msg("trigger file change detected")

			if triggerInfo.Type == trigger.Ignored {
				ignoreSet[triggeredStack] = struct{}{}
				continue
			}

			if triggerInfo.Type != trigger.Changed {
				printer.Stderr.Warnf("skipping unsupported trigger type: %s", triggerInfo.Type)
				continue
			}

			cfg, found := m.root.Lookup(triggeredStack)
			if !found || !cfg.IsStack() {
				logger.Debug().Msg("trigger path is not a stack, nothing to do")
				continue
			}

			s, err := config.NewStackFromHCL(m.root.HostDir(), cfg.Node)
			if err != nil {
				return nil, errors.E(errListChanged, err)
			}

			stackSet[s.Dir] = Entry{
				Stack:  s,
				Reason: "stack has been triggered by: " + projpath.String(),
			}
			continue
		}

		dirname := filepath.Dir(abspath)

		if _, ok := stackSet[project.PrjAbsPath(m.root.HostDir(), dirname)]; ok {
			continue
		}

		cfgpath := project.PrjAbsPath(m.root.HostDir(), dirname)
		stackTree, found := m.root.Lookup(cfgpath)
		if !found || !stackTree.IsStack() {
			checkdir := cfgpath
			// check if any parent directory is a stack
			for checkdir.String() != "/" {
				checkdir = checkdir.Dir()
				stackTree, found = m.root.Lookup(checkdir)
				if found && stackTree.IsStack() {
					break
				}
			}
			if !found || !stackTree.IsStack() {
				continue
			}
		}

		s, err := config.NewStackFromHCL(m.root.HostDir(), stackTree.Node)
		if err != nil {
			return nil, errors.E(errListChanged, err)
		}

		stackSet[s.Dir] = Entry{
			Stack:  s,
			Reason: "stack has unmerged changes",
		}
	}

	for ignored := range ignoreSet {
		delete(stackSet, ignored)
	}

	allstacks, err := m.allStacks()
	if err != nil {
		return nil, err
	}

	tgModulesMap := make(map[project.Path]*tg.Module)
	var tgModules tg.Modules

	if m.root.IsTerragruntChangeDetectionEnabled() {
		// discover Terragrunt modules
		tgModules, err = tg.ScanModules(m.root.HostDir(), project.NewPath("/"), false)
		if err != nil {
			return nil, errors.E(errListChanged, err, "scanning terragrunt modules")
		}

		for _, mod := range tgModules {
			tgModulesMap[mod.Path] = mod
		}
	}

rangeStacks:
	for _, stackEntry := range allstacks {
		stack := stackEntry.Stack
		if _, ok := stackSet[stack.Dir]; ok {
			continue
		}

		if changed, ok := hasChangedWatchedFiles(stack, m.cache.changedFiles); ok {
			logger.Debug().
				Stringer("stack", stack).
				Stringer("watchfile", changed).
				Msg("changed.")

			stack.IsChanged = true
			stackSet[stack.Dir] = Entry{
				Stack: stack,
				Reason: fmt.Sprintf(
					"stack changed because watched file %q changed",
					changed,
				),
			}
			continue rangeStacks
		}

		// Terraform module change detection
		err := m.filesApply(stack.Dir, func(fname string) error {
			if path.Ext(fname) != ".tf" {
				return nil
			}

			tfpath := filepath.Join(stack.HostDir(m.root), fname)

			modules, err := tf.ParseModules(tfpath)
			if err != nil {
				return errors.E(errListChanged, "parsing modules", err)
			}

			for _, mod := range modules {
				changed, why, err := m.tfModuleChanged(mod, stack.HostDir(m.root), gitBaseRef, make(map[string]bool))
				if err != nil {
					return errors.E(errListChanged, err, "checking module %q", mod.Source)
				}

				if changed {
					logger.Debug().
						Stringer("stack", stack).
						Str("configFile", tfpath).
						Msg("Module changed.")

					stack.IsChanged = true
					stackSet[stack.Dir] = Entry{
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
			return nil, errors.E(errListChanged, "checking if Terraform module changes", err)
		}

		// tgModulesMap is only populated if Terragrunt is enabled.
		tgMod, ok := tgModulesMap[stack.Dir]
		if !ok {
			continue
		}

		changed, why, err := m.tgModuleChanged(stack, tgMod, gitBaseRef, stackSet, tgModulesMap)
		if err != nil {
			return nil, errors.E(errListChanged, err, "checking if Terragrunt module changes")
		}

		if changed {
			logger.Debug().
				Stringer("stack", stack).
				Str("changed", tgMod.Source).
				Msg("Terragrunt module changed.")

			stack.IsChanged = true
			stackSet[stack.Dir] = Entry{
				Stack:  stack,
				Reason: fmt.Sprintf("stack changed because module %q changed because %s", tgMod.Path, why),
			}
			continue rangeStacks
		}
	}

	changedStacks := make([]Entry, 0, len(stackSet))
	for _, stack := range stackSet {
		changedStacks = append(changedStacks, stack)
	}

	sort.Sort(EntrySlice(changedStacks))

	return &Report{
		Checks: checks,
		Stacks: changedStacks,
	}, nil
}

func (m *Manager) allStacks() ([]Entry, error) {
	var allstacks []Entry
	if m.cache.stacks != nil {
		allstacks = m.cache.stacks
	} else {
		var err error
		allstacks, err = List(m.root, m.root.Tree())
		if err != nil {
			return nil, errors.E(errListChanged, "searching for stacks", err)
		}
		m.cache.stacks = allstacks
	}
	return allstacks, nil
}

// StackByID returns the stack with the given id.
func (m *Manager) StackByID(id string) (*config.Stack, bool, error) {
	report, err := m.List(false)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range report.Stacks {
		if entry.Stack.ID == id {
			return entry.Stack, true, nil
		}
	}
	return nil, false, nil
}

// AddWantedOf returns all wanted stacks from the given stacks.
func (m *Manager) AddWantedOf(scopeStacks config.List[*config.SortableStack]) (config.List[*config.SortableStack], error) {
	wantsDag := dag.New[*config.Stack]()
	allstacks, err := config.LoadAllStacks(m.root, m.root.Tree())
	if err != nil {
		return nil, errors.E(err, "loading all stacks")
	}

	visited := dag.Visited{}
	sort.Sort(allstacks)
	for _, elem := range allstacks {
		err := run.BuildDAG(
			wantsDag,
			m.root,
			elem.Stack,
			"wanted_by",
			func(s config.Stack) []string { return s.WantedBy },
			"wants",
			func(s config.Stack) []string { return s.Wants },
			visited,
		)

		if err != nil {
			return nil, errors.E(err, "building wants DAG")
		}
	}

	reason, err := wantsDag.Validate()
	if err != nil {
		if errors.IsKind(err, dag.ErrCycleDetected) {
			printer.Stderr.WarnWithDetails(
				"Stack selection clauses (wants/wanted_by) have cycles",
				errors.E(reason, err),
			)
		} else {
			printer.Stderr.WarnWithDetails(
				"Stack selection clauses (wants/wanted_by) have errors",
				err,
			)
		}
	}

	var selectedStacks config.List[*config.SortableStack]
	visited = dag.Visited{}
	addStack := func(s *config.Stack) {
		if _, ok := visited[dag.ID(s.Dir.String())]; ok {
			return
		}

		visited[dag.ID(s.Dir.String())] = struct{}{}
		selectedStacks = append(selectedStacks, s.Sortable())
	}

	var pending []dag.ID
	for _, s := range scopeStacks {
		pending = append(pending, dag.ID(s.Dir().String()))
	}

	for len(pending) > 0 {
		id := pending[0]
		s, _ := wantsDag.Node(id)

		addStack(s)
		pending = pending[1:]

		ancestors := wantsDag.AncestorsOf(id)
		for _, id := range ancestors {
			if _, ok := visited[id]; !ok {
				pending = append(pending, id)
			}
		}
	}
	return selectedStacks, nil
}

func (m *Manager) filesApply(dir project.Path, apply func(fname string) error) (err error) {
	tree, ok := m.root.Lookup(dir)
	if !ok {
		return errors.E("directory not found: %s", dir)
	}

	for _, fname := range tree.OtherFiles {
		err := apply(fname)
		if err != nil {
			return errors.E(err, "applying operation to file %q", fname)
		}
	}
	return nil
}

// tfModuleChanged recursively check if the Terraform module mod or any of the modules it
// uses has changed. All .tf files of the module are parsed and this function is
// called recursively. The visited keep track of the modules already parsed to
// avoid infinite loops.
func (m *Manager) tfModuleChanged(
	mod tf.Module, basedir string, gitBaseRef string, visited map[string]bool,
) (changed bool, why string, err error) {
	if _, ok := visited[mod.Source]; ok {
		return false, "", nil
	}

	if !mod.IsLocal() {
		// if the source is a remote path (URL, VCS path, S3 bucket, etc) then
		// we assume it's not changed.
		return false, "", nil
	}

	modAbsPath := filepath.Join(basedir, mod.Source)
	modPath := project.PrjAbsPath(m.root.HostDir(), modAbsPath)

	st, err := os.Stat(modAbsPath)

	// TODO(i4k): resolve symlinks

	if err != nil || !st.IsDir() {
		return false, "", errors.E("\"source\" path %q is not a directory", modAbsPath)
	}

	for _, changedFile := range m.cache.changedFiles {
		changedPath := filepath.Join(m.root.HostDir(), changedFile)
		if strings.HasPrefix(changedPath, modAbsPath) {
			return true, fmt.Sprintf("module %q has unmerged changes", mod.Source), nil
		}
	}

	visited[mod.Source] = true

	err = m.filesApply(modPath, func(fname string) error {
		if changed {
			return nil
		}
		if path.Ext(fname) != ".tf" {
			return nil
		}

		modules, err := tf.ParseModules(filepath.Join(modAbsPath, fname))
		if err != nil {
			return errors.E(err, "parsing module %q", mod.Source)
		}

		for _, mod2 := range modules {
			var reason string

			changed, reason, err = m.tfModuleChanged(mod2, modAbsPath, gitBaseRef, visited)
			if err != nil {
				return err
			}

			if changed {
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

func (m *Manager) tgModuleChanged(
	stack *config.Stack, tgMod *tg.Module, gitBaseRef string, stackSet map[project.Path]Entry, tgModuleMap map[project.Path]*tg.Module,
) (changed bool, why string, err error) {
	tfMod := tf.Module{Source: tgMod.Source}
	if tfMod.IsLocal() {
		changed, why, err := m.tfModuleChanged(tfMod, project.AbsPath(m.root.HostDir(), tgMod.Path.String()), gitBaseRef, make(map[string]bool))
		if err != nil {
			return false, "", errors.E(errListChanged, err, "checking if Terraform module changes (in Terragrunt context)")
		}
		if changed {
			return true, fmt.Sprintf("module %q changed because %s", tgMod.Path, why), nil
		}
	}

	for _, dep := range tgMod.DependsOn {
		// if the module is a stack already detected as changed, just mark this as changed and
		// move on. Fast path.
		depStack, found := m.root.Lookup(dep)
		if found && depStack.IsStack() {
			if _, ok := stackSet[depStack.Dir()]; ok {
				return true, fmt.Sprintf("module %q changed because %q changed", tgMod.Path, dep), nil
			}
		}

		for _, changedFile := range m.cache.changedFiles {
			changedPath := project.PrjAbsPath(m.root.HostDir(), changedFile)
			if dep == changedPath {
				return true, fmt.Sprintf("module %q changed because %q changed", tgMod.Path, dep), nil
			}
		}

		depAbsPath := project.AbsPath(m.root.HostDir(), dep.String())
		// if the dep is a directory, check if it changed
		info, err := os.Lstat(depAbsPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return false, "", errors.E(errListChanged, "checking if Terragrunt module changes", err)
		}
		if !info.IsDir() {
			// if it's not a directory, then if changed it shall have been detected by the changedFiles.
			continue
		}

		for _, file := range m.cache.changedFiles {
			if strings.HasPrefix(filepath.Join(m.root.HostDir(), file), depAbsPath) {
				return true, fmt.Sprintf("module %q changed because %q changed", tgMod.Path, dep), nil
			}
		}

		// if the dep is a Terragrunt module, check if it changed
		depTgMod, ok := tgModuleMap[dep]
		if ok {
			changed, why, err := m.tgModuleChanged(stack, depTgMod, gitBaseRef, stackSet, tgModuleMap)
			if err != nil {
				return false, "", errors.E(errListChanged, "checking if Terragrunt module changes", err)
			}
			if changed {
				return true, fmt.Sprintf("module %q changed because %q changed because %s", tgMod.Path, dep, why), nil
			}
		}
	}

	return false, "", nil
}

// listChangedFiles lists all changed files in the dir directory.
func (m *Manager) listChangedFiles(dir string, gitBaseRef string) ([]string, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, errors.E(err, "stat failed on %q", dir)
	}

	if !st.IsDir() {
		return nil, errors.E("is not a directory")
	}

	dirWrapper := m.git.With().WorkingDir(dir).Wrapper()

	baseRef, err := dirWrapper.RevParse(gitBaseRef)
	if err != nil {
		return nil, errors.E(err, "getting revision %q", gitBaseRef)
	}

	headRef, err := dirWrapper.RevParse("HEAD")
	if err != nil {
		return nil, errors.E(err, "getting HEAD revision")
	}

	if baseRef == headRef {
		return []string{}, nil
	}

	return dirWrapper.DiffNames(baseRef, headRef)
}

func hasChangedWatchedFiles(stack *config.Stack, changedFiles []string) (project.Path, bool) {
	for _, watchFile := range stack.Watch {
		for _, file := range changedFiles {
			if file == watchFile.String()[1:] { // project paths
				return watchFile, true
			}
		}
	}
	return project.Path{}, false
}

func checkRepoIsClean(g *git.Git) (RepoChecks, error) {
	untracked, err := g.ListUntracked()
	if err != nil {
		return RepoChecks{}, errors.E(err, "listing untracked files")
	}

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
func (x EntrySlice) Less(i, j int) bool { return x[i].Stack.Dir.String() < x[j].Stack.Dir.String() }
func (x EntrySlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

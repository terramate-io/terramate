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

		hasTerragruntExperimentsEnabled bool
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
		root:                            root,
		git:                             git,
		hasTerragruntExperimentsEnabled: root.HasExperiment("terragrunt"),
	}
}

// List walks the basedir directory looking for terraform stacks.
// It returns a lexicographic sorted list of stack directories.
func (m *Manager) List() (*Report, error) {
	entries, err := List(m.root.Tree())
	if err != nil {
		return nil, err
	}

	report := &Report{
		Stacks: entries,
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

// ListChanged lists the stacks that have changed on the current branch,
// compared to the main branch. This method assumes a version control
// system in place and that you are working on a branch that is not main.
// It's an error to call this method in a directory that's not
// inside a repository or a repository with no commits in it.
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

	changedFiles, err := m.listChangedFiles(m.root.HostDir(), gitBaseRef)
	if err != nil {
		return nil, errors.E(errListChanged, err)
	}

	stackSet := map[project.Path]Entry{}

	for _, path := range changedFiles {
		abspath := filepath.Join(m.root.HostDir(), path)
		projpath := project.PrjAbsPath(m.root.HostDir(), abspath)
		triggeredStack, isTriggerFile := trigger.StackPath(projpath)

		logger = logger.With().
			Stringer("path", projpath).
			Logger()

		if isTriggerFile {
			logger = logger.With().
				Stringer("trigger", triggeredStack).
				Logger()

			logger.Debug().Msg("trigger file change detected")

			if _, err := os.Stat(abspath); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					logger.Debug().Msg("ignoring deleted trigger file")
					continue
				}
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

	allstacks, err := List(m.root.Tree())
	if err != nil {
		return nil, errors.E(errListChanged, "searching for stacks", err)
	}

	// discover Terragrunt modules
	var tgModules tg.Modules
	tgModulesMap := make(map[project.Path]*tg.Module)
	if m.hasTerragruntExperimentsEnabled {
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

		if changed, ok := hasChangedWatchedFiles(stack, changedFiles); ok {
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
		err := m.filesApply(stack.HostDir(m.root), func(file fs.DirEntry) error {
			if path.Ext(file.Name()) != ".tf" {
				return nil
			}

			tfpath := filepath.Join(stack.HostDir(m.root), file.Name())

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

		// Terragrunt module change detection
		if !m.hasTerragruntExperimentsEnabled {
			continue
		}

		tgMod, ok := tgModulesMap[stack.Dir]
		if !ok {
			continue
		}

		changed, why, err := m.tgModuleChanged(stack, tgMod, gitBaseRef, changedFiles, stackSet, tgModulesMap)
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

// AddWantedOf returns all wanted stacks from the given stacks.
func (m *Manager) AddWantedOf(scopeStacks config.List[*config.SortableStack]) (config.List[*config.SortableStack], error) {
	wantsDag := dag.New[*config.Stack]()
	allstacks, err := config.LoadAllStacks(m.root.Tree())
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

func (m *Manager) filesApply(dir string, apply func(file fs.DirEntry) error) (err error) {
	f, err := os.Open(dir)
	if err != nil {
		return errors.E(err, "opening directory %q", dir)
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	files, err := f.ReadDir(-1)
	if err != nil {
		return errors.E(err, "listing files of directory %q", dir)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		err := apply(file)
		if err != nil {
			return errors.E(err, "applying operation to file %q", file.Name())
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

	modPath := filepath.Join(basedir, mod.Source)

	st, err := os.Stat(modPath)

	// TODO(i4k): resolve symlinks

	if err != nil || !st.IsDir() {
		return false, "", errors.E("\"source\" path %q is not a directory", modPath)
	}

	changedFiles, err := m.listChangedFiles(modPath, gitBaseRef)
	if err != nil {
		return false, "", errors.E(err,
			"listing changes in the module %q",
			mod.Source)
	}

	if len(changedFiles) > 0 {
		return true, fmt.Sprintf("module %q has unmerged changes", mod.Source), nil
	}

	visited[mod.Source] = true

	err = m.filesApply(modPath, func(file fs.DirEntry) error {
		if changed {
			return nil
		}
		if path.Ext(file.Name()) != ".tf" {
			return nil
		}

		modules, err := tf.ParseModules(filepath.Join(modPath, file.Name()))
		if err != nil {
			return errors.E(err, "parsing module %q", mod.Source)
		}

		for _, mod2 := range modules {
			var reason string

			changed, reason, err = m.tfModuleChanged(mod2, modPath, gitBaseRef, visited)
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
	stack *config.Stack, tgMod *tg.Module, gitBaseRef string, changedFiles []string, stackSet map[project.Path]Entry, tgModuleMap map[project.Path]*tg.Module,
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

		for _, changedFile := range changedFiles {
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

		changedFiles, err := m.listChangedFiles(depAbsPath, gitBaseRef)
		if err != nil {
			return false, "", errors.E(errListChanged, "checking if Terragrunt module changes", err)
		}
		if len(changedFiles) > 0 {
			return true, fmt.Sprintf("module %q changed because %q changed", tgMod.Path, dep), nil
		}

		// if the dep is a Terragrunt module, check if it changed
		depTgMod, ok := tgModuleMap[dep]
		if ok {
			changed, why, err := m.tgModuleChanged(stack, depTgMod, gitBaseRef, changedFiles, stackSet, tgModuleMap)
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

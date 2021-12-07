package terrastack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/mineiros-io/terrastack/git"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
)

type (
	// Manager is the terrastack stacks manager.
	Manager struct {
		basedir    string // basedir is the stacks base directory.
		gitBaseRef string // gitBaseRef is the git ref where we compare changes.

		parser hcl.ModuleParser
	}

	// Entry is a generic directory entry result.
	Entry struct {
		Dir    string
		Reason string // Reason why this entry was returned.
	}
)

// NewManager creates a new stack manager. The basedir is the base directory
// where all stacks reside inside.
func NewManager(basedir string, gitBaseRef string) *Manager {
	return &Manager{
		basedir:    basedir,
		gitBaseRef: gitBaseRef,
		parser:     hhcl.NewParser(),
	}
}

// List walks the basedir directory looking for terraform stacks.
// It returns a lexicographic sorted list of stack directories.
func (m *Manager) List() ([]Entry, error) {
	return ListStacks(m.basedir)
}

// ListChanged lists the stacks that have changed on the current branch,
// compared to the main branch. This method assumes a version control
// system in place and that you are working on a branch that is not main.
// It's an error to call this method in a directory that's not
// inside a repository or a repository with no commits in it.
func (m *Manager) ListChanged() ([]Entry, error) {
	stackSet := map[string]Entry{}
	files, err := listChangedFiles(m.basedir, m.gitBaseRef)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		dirname := filepath.Dir(filepath.Join(m.basedir, path))
		if _, ok := stackSet[dirname]; !ok && isStack(dirname) {
			stackSet[dirname] = Entry{
				Dir:    dirname,
				Reason: "stack has unmerged changes",
			}
		}
	}

	allstacks, err := m.List()
	if err != nil {
		return nil, fmt.Errorf("searching for stacks: %v", err)
	}

	for _, stack := range allstacks {
		if _, ok := stackSet[stack.Dir]; ok {
			continue
		}

		err := m.filesApply(stack.Dir, func(file fs.DirEntry) error {
			if path.Ext(file.Name()) != ".tf" {
				return nil
			}

			tfpath := filepath.Join(stack.Dir, file.Name())
			modules, err := m.parser.ParseModules(tfpath)
			if err != nil {
				return fmt.Errorf("parsing modules at %q: %w",
					file.Name(), err)
			}

			for _, mod := range modules {
				changed, why, err := m.moduleChanged(mod, stack.Dir, make(map[string]bool))
				if err != nil {
					return fmt.Errorf("checking module %q: %w", mod.Source, err)
				}

				if changed {
					stackSet[stack.Dir] = Entry{
						Dir:    stack.Dir,
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

	changedStacks := make([]Entry, 0, len(stackSet))
	for _, stack := range stackSet {
		changedStacks = append(changedStacks, stack)
	}

	sort.Sort(EntrySlice(changedStacks))

	return changedStacks, nil
}

func (m *Manager) filesApply(dir string, apply func(file fs.DirEntry) error) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("listing files of directory %q: %w", dir, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		err := apply(file)
		if err != nil {
			return fmt.Errorf("applying operation to file %q: %w", file, err)
		}
	}

	return nil
}

// listChangedFiles lists all changed files in the dir directory.
func listChangedFiles(dir string, gitBaseRef string) ([]string, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	if !st.IsDir() {
		return nil, fmt.Errorf("is not a directory")
	}

	g, err := git.WithConfig(git.Config{
		WorkingDir: dir,
	})
	if err != nil {
		return nil, err
	}

	if !g.IsRepository() {
		return nil, fmt.Errorf("the path \"%s\" is not a git repository", dir)
	}

	untracked, err := g.ListUntracked()
	if err != nil {
		return nil, fmt.Errorf("listing untracked files: %v", err)
	}

	if len(untracked) > 0 {
		return nil, fmt.Errorf("repository has untracked files: %v", untracked)
	}

	uncommitted, err := g.ListUncommitted()
	if err != nil {
		return nil, fmt.Errorf("listing uncommitted files: %v", err)
	}

	if len(uncommitted) > 0 {
		return nil, fmt.Errorf("repository has uncommitted files: %v", uncommitted)
	}

	baseRef, err := g.RevParse(gitBaseRef)
	if err != nil {
		return nil, fmt.Errorf("getting revision %q: %w", gitBaseRef, err)
	}

	headRef, err := g.RevParse("HEAD")
	if err != nil {
		return nil, fmt.Errorf("getting HEAD revision: %w", err)
	}

	if baseRef == headRef {
		return []string{}, nil
	}

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

// moduleChanged recursively check if the module mod or any of the modules it
// uses has changed. All .tf files of the module are parsed and this function is
// called recursively. The visited keep track of the modules already parsed to
// avoid infinite loops.
func (m *Manager) moduleChanged(
	mod hcl.Module, basedir string, visited map[string]bool,
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
		// terrastack is not a TF linter so if the module source is not
		// reachable or is not a directory, for any reason, we do not fail.

		return false, "", nil
	}

	changedFiles, err := listChangedFiles(modPath, m.gitBaseRef)
	if err != nil {
		return false, "", fmt.Errorf("listing changes in the module %q: %w",
			mod.Source, err)
	}

	if len(changedFiles) > 0 {
		return true, fmt.Sprintf("module %q has unmerged changes", mod.Source), nil
	}

	visited[mod.Source] = true
	err = m.filesApply(modPath, func(file fs.DirEntry) error {
		if path.Ext(file.Name()) != ".tf" {
			return nil
		}

		modules, err := m.parser.ParseModules(filepath.Join(modPath, file.Name()))
		if err != nil {
			return fmt.Errorf("parsing module %q: %w", mod.Source, err)
		}

		for _, mod2 := range modules {
			var reason string
			changed, reason, err = m.moduleChanged(mod2, modPath, visited)
			if err != nil {
				return err
			}

			if changed {
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

func isStack(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return false
	}

	fname := filepath.Join(dir, ConfigFilename)
	st, err = os.Stat(fname)
	if err != nil {
		return false
	}

	return st.Mode().IsRegular()
}

// EntrySlice implements the Sort interface.
type EntrySlice []Entry

func (x EntrySlice) Len() int           { return len(x) }
func (x EntrySlice) Less(i, j int) bool { return x[i].Dir < x[j].Dir }
func (x EntrySlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

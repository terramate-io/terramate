package terrastack

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terrastack/git"
)

var (

	//go:embed VERSION
	version string
)

// Version of terrastack.
func Version() string { return strings.TrimSpace(version) }

// List walks the basedir directory looking for terraform stacks.
// It returns a sorted list of stack directories.
func List(basedir string) ([]string, error) {
	paths := []string{}

	err := filepath.Walk(
		basedir,
		func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				stackfile := filepath.Join(path, ConfigFilename)
				st, err := os.Stat(stackfile)
				if err != nil || !st.Mode().IsRegular() {
					return nil
				}

				paths = append(paths, path)
			}

			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("while walking dir: %w", err)
	}

	sort.Strings(paths)

	return paths, nil
}

// ListChanged lists the stacks that have changed since last merge in the main
// branch. It's an error to invoke this function in a directory that's not
// inside a repository or a repository with no commits in it.
func ListChanged(basedir string) ([]string, error) {
	st, err := os.Stat(basedir)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %w", err)
	}

	if !st.IsDir() {
		return nil, fmt.Errorf("is not a directory")
	}

	g, err := git.WithConfig(git.Config{
		WorkingDir: basedir,
	})
	if err != nil {
		return nil, err
	}

	if !g.IsRepository() {
		return nil, fmt.Errorf("the path %q is not a git repository", basedir)
	}

	mainRef, err := g.RevParse("main")
	if err != nil {
		return nil, fmt.Errorf("getting main revision: %w", err)
	}

	headRef, err := g.RevParse("HEAD")
	if err != nil {
		return nil, fmt.Errorf("getting HEAD revision: %w", err)
	}

	mergeBase, err := g.MergeBase("HEAD", "main")
	if err != nil {
		return nil, fmt.Errorf("getting merge-base HEAD main: %w", err)
	}

	changeBase := mainRef

	if mainRef == headRef {
		return []string{}, nil
	}

	if mainRef != mergeBase {
		return nil, fmt.Errorf("main branch is not reachable: main %q != merge %q",
			mainRef, mergeBase)
	}

	if g.IsDirty() {
		return nil, fmt.Errorf("repository has uncommited changes")
	}

	diff, err := g.DiffTree(changeBase, headRef, true, true)
	if err != nil {
		return nil, fmt.Errorf("running git diff %q: %w", changeBase, err)
	}

	stackSet := map[string]struct{}{}
	files := strings.Split(diff, "\n")
	stacks := make([]string, 0, len(files))

	for _, f := range files {
		dirname := filepath.Dir(filepath.Join(basedir, f))
		if _, ok := stackSet[dirname]; !ok && isStack(dirname) {
			stackSet[dirname] = struct{}{}
		}
	}

	for stack := range stackSet {
		stacks = append(stacks, stack)
	}

	return stacks, nil
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

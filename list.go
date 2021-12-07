package terrastack

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ListStacks walks the basedir directory looking for terraform stacks.
// It returns a lexicographic sorted list of stack directories.
func ListStacks(basedir string) ([]Entry, error) {
	entries := []Entry{}

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

				entries = append(entries, Entry{Dir: path})
			}

			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("while walking dir: %w", err)
	}

	return entries, nil
}

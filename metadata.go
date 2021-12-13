package terramate

import (
	"path/filepath"
	"strings"
)

// StackMetadata has all metadata loaded per stack
type StackMetadata struct {
	Name string
	Path string
}

// Metadata has all metadata loader per project
type Metadata struct {
	// Stacks is a lexycographicaly sorted (by stack path) list of stack metadata
	Stacks []StackMetadata
}

// LoadMetadata loads the project metadata given the project basedir.
func LoadMetadata(basedir string) (Metadata, error) {
	stacks, err := ListStacks(basedir)
	if err != nil {
		return Metadata{}, err
	}

	stacksMetadata := make([]StackMetadata, len(stacks))
	for i, stack := range stacks {
		stacksMetadata[i] = StackMetadata{
			Name: filepath.Base(stack.Dir),
			Path: strings.TrimPrefix(stack.Dir, basedir),
		}
	}

	return Metadata{
		Stacks: stacksMetadata,
	}, nil
}

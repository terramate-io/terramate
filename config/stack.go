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

package config

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

type (
	// Stack represents an evaluated stack.
	Stack struct {
		// Dir is project's stack directory.
		Dir project.Path

		// ID of the stack.
		ID string

		// Name of the stack.
		Name string

		// Description is the description of the stack.
		Description string

		// Tags is the list of tags of the stack.
		// A tag
		Tags []string

		// After is a list of stack paths that must run before this stack.
		After []string

		// Before is a list of stack paths that must run after this stack.
		Before []string

		// Wants is the list of stacks that must be selected whenever this stack
		// is selected.
		Wants []string

		// wantedBy is the list of stacks that must select this stack
		// whenever they are selected.
		WantedBy []string

		// Watch is the list of files to be watched for changes.
		Watch []project.Path

		// IsChanged tells if this is a changed stack.
		IsChanged bool
	}

	// SortableStack is a wrapper for the Stack which implements the [DirElem] type.
	SortableStack struct {
		*Stack
	}
)

const (
	// ErrStackDuplicatedID indicates that two or more stacks have the same ID.
	ErrStackDuplicatedID errors.Kind = "duplicated ID found on stacks"

	// ErrStackInvalidWatch indicates the stack.watch attribute contains invalid values.
	ErrStackInvalidWatch errors.Kind = "invalid stack.watch attribute"

	// ErrStackInvalidTag indicates the stack.tags is invalid.
	ErrStackInvalidTag errors.Kind = "invalid stack.tags entry"
)

// NewStackFromHCL creates a new stack from raw configuration cfg.
func NewStackFromHCL(root string, cfg hcl.Config) (*Stack, error) {
	name := cfg.Stack.Name
	if name == "" {
		name = filepath.Base(cfg.AbsDir())
	}

	watchFiles, err := validateWatchPaths(root, cfg.AbsDir(), cfg.Stack.Watch)
	if err != nil {
		return nil, errors.E(err, ErrStackInvalidWatch)
	}

	stack := &Stack{
		Name:        name,
		ID:          cfg.Stack.ID,
		Description: cfg.Stack.Description,
		Tags:        cfg.Stack.Tags,
		After:       cfg.Stack.After,
		Before:      cfg.Stack.Before,
		Wants:       cfg.Stack.Wants,
		WantedBy:    cfg.Stack.WantedBy,
		Watch:       watchFiles,
		Dir:         project.PrjAbsPath(root, cfg.AbsDir()),
	}
	err = stack.Validate()
	if err != nil {
		return nil, err
	}
	return stack, nil
}

// Validate if all stack fields are correct.
func (s Stack) Validate() error {
	return s.validateTags()
}

func (s Stack) validateTags() error {
	for _, tag := range s.Tags {
		for i, r := range tag {
			switch i {
			case 0:
				if !isLowerAlpha(r) {
					return errors.E(
						ErrStackInvalidTag,
						"%q: tags must start with lowercase alphabetic character ([a-z])",
						tag)
				}
			case len(tag) - 1: // last rune
				if !isLowerAlnum(r) {
					return errors.E(
						ErrStackInvalidTag,
						"%q: tags must end with lowercase alphanumeric ([0-9a-z]+)",
						tag)
				}
			default:
				if !isLowerAlnum(r) && r != '-' && r != '_' {
					return errors.E(
						ErrStackInvalidTag,
						"%q: [a-z_-] are the only permitted characters in tags",
						tag)
				}
			}
		}
	}
	return nil
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLowerAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z')
}
func isLowerAlnum(r rune) bool {
	return isLowerAlpha(r) || isDigit(r)
}

// AppendBefore appends the path to the list of stacks that must run after this
// stack.
func (s *Stack) AppendBefore(path string) {
	s.Before = append(s.Before, path)
}

// String representation of the stack.
func (s *Stack) String() string { return s.Dir.String() }

// PathBase returns the base name of the stack path.
func (s *Stack) PathBase() string { return filepath.Base(s.Dir.String()) }

// RelPath returns the project's relative path of stack.
func (s *Stack) RelPath() string { return s.Dir.String()[1:] }

// RelPathToRoot returns the relative path from the stack to root.
func (s *Stack) RelPathToRoot(root *Root) string {
	// should never fail as abspath is constructed inside rootdir.
	rel, _ := filepath.Rel(s.HostDir(root), root.HostDir())
	return filepath.ToSlash(rel)
}

// HostDir returns the file system absolute path of stack.
func (s *Stack) HostDir(root *Root) string {
	return project.AbsPath(root.HostDir(), s.Dir.String())
}

// RuntimeValues returns the runtime "terramate" namespace for the stack.
func (s *Stack) RuntimeValues(root *Root) map[string]cty.Value {
	logger := log.With().
		Str("action", "stack.stackMetaToCtyMap()").
		Logger()

	logger.Trace().Msg("creating stack metadata")

	stackpath := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(s.Dir.String()),
		"relative": cty.StringVal(s.RelPath()),
		"basename": cty.StringVal(s.PathBase()),
		"to_root":  cty.StringVal(s.RelPathToRoot(root)),
	})
	stackMapVals := map[string]cty.Value{
		"name":        cty.StringVal(s.Name),
		"description": cty.StringVal(s.Description),
		"tags":        toCtyStringList(s.Tags),
		"path":        stackpath,
	}
	if s.ID != "" {
		logger.Trace().
			Str("id", s.ID).
			Msg("adding stack ID to metadata")

		stackMapVals["id"] = cty.StringVal(s.ID)
	}
	stack := cty.ObjectVal(stackMapVals)
	return map[string]cty.Value{
		"name":        cty.StringVal(s.Name),         // DEPRECATED
		"path":        cty.StringVal(s.Dir.String()), // DEPRECATED
		"description": cty.StringVal(s.Description),  // DEPRECATED
		"stack":       stack,
	}
}

// Sortable returns an implementation of stack which can be sorted by [config.List].
func (s *Stack) Sortable() *SortableStack {
	return &SortableStack{
		Stack: s,
	}
}

func validateWatchPaths(rootdir string, stackpath string, paths []string) (project.Paths, error) {
	var projectPaths project.Paths
	for _, pathstr := range paths {
		var abspath string
		if path.IsAbs(pathstr) {
			abspath = filepath.Join(rootdir, filepath.FromSlash(pathstr))
		} else {
			abspath = filepath.Join(stackpath, filepath.FromSlash(pathstr))
		}
		if !strings.HasPrefix(abspath, rootdir) {
			return nil, errors.E("path %s is outside project root", pathstr)
		}
		st, err := os.Stat(abspath)
		if err == nil {
			if st.IsDir() {
				return nil, errors.E("stack.watch must be a list of regular files "+
					"but directory %q was provided", pathstr)
			}

			if !st.Mode().IsRegular() {
				return nil, errors.E("stack.watch must be a list of regular files "+
					"but file %q has mode %s", pathstr, st.Mode())
			}
		}
		projectPaths = append(projectPaths, project.PrjAbsPath(rootdir, abspath))
	}
	return projectPaths, nil
}

// StacksFromTrees converts a List[*Tree] into a List[*Stack].
func StacksFromTrees(root string, trees List[*Tree]) (List[*SortableStack], error) {
	var stacks List[*SortableStack]
	for _, tree := range trees {
		s, err := NewStackFromHCL(root, tree.Node)
		if err != nil {
			return List[*SortableStack]{}, err
		}
		stacks = append(stacks, &SortableStack{s})
	}
	return stacks, nil
}

// LoadAllStacks loads all stacks inside the given rootdir.
func LoadAllStacks(cfg *Tree) (List[*SortableStack], error) {
	logger := log.With().
		Str("action", "stack.LoadAll()").
		Str("root", cfg.RootDir()).
		Logger()

	stacks := List[*SortableStack]{}
	stacksIDs := map[string]*Stack{}

	for _, stackNode := range cfg.Stacks() {
		stack, err := NewStackFromHCL(cfg.RootDir(), stackNode.Node)
		if err != nil {
			return List[*SortableStack]{}, err
		}

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Debug().Msg("Found stack")
		stacks = append(stacks, stack.Sortable())

		if stack.ID != "" {
			logger.Trace().Msg("stack has ID, checking for duplicate")
			if otherStack, ok := stacksIDs[stack.ID]; ok {
				return List[*SortableStack]{}, errors.E(ErrStackDuplicatedID,
					"stack %q and %q have same ID %q",
					stack.Dir,
					otherStack.Dir,
					stack.ID,
				)
			}
			stacksIDs[stack.ID] = stack
		}
	}

	return stacks, nil
}

// LoadStack a single stack from dir.
func LoadStack(root *Root, dir project.Path) (*Stack, error) {
	node, ok := root.Lookup(dir)
	if !ok {
		return nil, errors.E("config not found at %s", dir)
	}
	if !node.IsStack() {
		return nil, errors.E("config at %s is not a stack")
	}
	return NewStackFromHCL(root.HostDir(), node.Node)
}

// TryLoadStack tries to load a single stack from dir. It sets found as true in case
// the stack was successfully loaded.
func TryLoadStack(root *Root, cfgdir project.Path) (stack *Stack, found bool, err error) {
	tree, ok := root.Lookup(cfgdir)
	if !ok {
		return nil, false, nil
	}

	if !tree.IsStack() {
		return nil, false, nil
	}

	s, err := NewStackFromHCL(root.HostDir(), tree.Node)
	if err != nil {
		return nil, true, err
	}
	return s, true, nil
}

// ReverseStacks reverses the given stacks slice.
func ReverseStacks(stacks List[*SortableStack]) {
	i, j := 0, len(stacks)-1
	for i < j {
		stacks[i], stacks[j] = stacks[j], stacks[i]
		i++
		j--
	}
}

// Paths returns the project paths from the list.
func (l List[T]) Paths() project.Paths {
	paths := make(project.Paths, len(l))
	for i, s := range l {
		paths[i] = s.Dir()
	}
	return paths
}

// Dir implements the [List] type.
func (s SortableStack) Dir() project.Path { return s.Stack.Dir }

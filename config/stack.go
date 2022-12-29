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
		// dir is the absolute dir of the stack relative to project's root.
		dir project.Path

		// ID of the stack.
		id hcl.StackID

		// name of the stack.
		name string

		// desc is the description of the stack.
		desc string

		// after is a list of stack paths that must run before this stack.
		after []string

		// before is a list of stack paths that must run after this stack.
		before []string

		// wants is the list of stacks that must be selected whenever this stack
		// is selected.
		wants []string

		// wantedBy is the list of stacks that must select this stack
		// whenever they are selected.
		wantedBy []string

		// watch is the list of files to be watched for changes.
		watch []project.Path

		// changed tells if this is a changed stack.
		changed bool
	}
)

const (
	// ErrStackDuplicatedID indicates that two or more stacks have the same ID.
	ErrStackDuplicatedID errors.Kind = "duplicated ID found on stacks"

	// ErrStackInvalidWatch indicates the stack.watch attribute contains invalid values.
	ErrStackInvalidWatch errors.Kind = "invalid stack.watch attribute"
)

// ensure we get a compiler error if stack doesn't implement errors.StackMeta.
var _ errors.StackMeta = &Stack{}

// NewStack creates a new stack from raw configuration cfg.
func NewStack(root string, cfg hcl.Config) (*Stack, error) {
	name := cfg.Stack.Name
	if name == "" {
		name = filepath.Base(cfg.AbsDir())
	}

	watchFiles, err := validateWatchPaths(root, cfg.AbsDir(), cfg.Stack.Watch)
	if err != nil {
		return nil, errors.E(err, ErrStackInvalidWatch)
	}

	return &Stack{
		name:     name,
		id:       cfg.Stack.ID,
		desc:     cfg.Stack.Description,
		after:    cfg.Stack.After,
		before:   cfg.Stack.Before,
		wants:    cfg.Stack.Wants,
		wantedBy: cfg.Stack.WantedBy,
		watch:    watchFiles,
		dir:      project.PrjAbsPath(root, cfg.AbsDir()),
	}, nil
}

// ID of the stack if it has one, or empty string and false otherwise.
func (s *Stack) ID() (string, bool) {
	return s.id.Value()
}

// Name of the stack.
func (s *Stack) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.Dir().String()
}

// Desc is the description of the stack.
func (s *Stack) Desc() string { return s.desc }

// After specifies the list of stacks that must run before this stack.
func (s Stack) After() []string { return s.after }

// Before specifies the list of stacks that must run after this stack.
func (s Stack) Before() []string { return s.before }

// AppendBefore appends the path to the list of stacks that must run after this
// stack.
func (s *Stack) AppendBefore(path string) {
	s.before = append(s.before, path)
}

// Wants specifies the list of wanted stacks.
func (s Stack) Wants() []string { return s.wants }

// WantedBy specifies the list of stacks that wants this stack.
func (s Stack) WantedBy() []string { return s.wantedBy }

// Watch returns the list of watched files.
func (s *Stack) Watch() []project.Path { return s.watch }

// IsChanged tells if the stack is marked as changed.
func (s *Stack) IsChanged() bool { return s.changed }

// SetChanged sets the changed flag of the stack.
func (s *Stack) SetChanged(b bool) { s.changed = b }

// String representation of the stack.
func (s *Stack) String() string { return s.Dir().String() }

// Dir is the directory of the stack.
func (s *Stack) Dir() project.Path { return s.dir }

// PathBase returns the base name of the stack path.
func (s *Stack) PathBase() string { return filepath.Base(s.dir.String()) }

// RelPath returns the project's relative path of stack.
func (s *Stack) RelPath() string { return s.dir.String()[1:] }

// RelPathToRoot returns the relative path from the stack to root.
func (s *Stack) RelPathToRoot(root *Root) string {
	// should never fail as abspath is constructed inside rootdir.
	rel, _ := filepath.Rel(s.HostDir(root), root.HostDir())
	return filepath.ToSlash(rel)
}

// HostDir returns the file system absolute path of stack.
func (s *Stack) HostDir(root *Root) string {
	return project.AbsPath(root.HostDir(), s.Dir().String())
}

func (m *Stack) stackMetaToCtyMap(root *Root) map[string]cty.Value {
	logger := log.With().
		Str("action", "stack.stackMetaToCtyMap()").
		Logger()

	logger.Trace().Msg("creating stack metadata")

	stackpath := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(m.Dir().String()),
		"relative": cty.StringVal(m.RelPath()),
		"basename": cty.StringVal(m.PathBase()),
		"to_root":  cty.StringVal(m.RelPathToRoot(root)),
	})
	stackMapVals := map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),
		"description": cty.StringVal(m.Desc()),
		"path":        stackpath,
	}
	if id, ok := m.ID(); ok {
		logger.Trace().
			Str("id", id).
			Msg("adding stack ID to metadata")

		stackMapVals["id"] = cty.StringVal(id)
	}
	stack := cty.ObjectVal(stackMapVals)
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),         // DEPRECATED
		"path":        cty.StringVal(m.Dir().String()), // DEPRECATED
		"description": cty.StringVal(m.Desc()),         // DEPRECATED
		"stack":       stack,
	}
}

// MetadataToCtyValues converts the metadatas to a map of cty.Values.
func (s *Stack) ToCtyValues(root *Root, projmeta project.Metadata) map[string]cty.Value {
	projvalues := projmeta.ToCtyMap()
	stackvalues := s.stackMetaToCtyMap(root)

	tmvar := map[string]cty.Value{}
	for k, v := range projvalues {
		tmvar[k] = v
	}
	for k, v := range stackvalues {
		if _, ok := tmvar[k]; ok {
			panic("project metadata and stack metadata conflicts")
		}
		tmvar[k] = v
	}
	return tmvar
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
func StacksFromTrees(root string, trees List[*Tree]) (List[*Stack], error) {
	var stacks List[*Stack]
	for _, tree := range trees {
		s, err := NewStack(root, tree.Node)
		if err != nil {
			return List[*Stack]{}, err
		}
		stacks = append(stacks, s)
	}
	return stacks, nil
}

// NewProjectMetadata creates project metadata from a given rootdir and a list of stacks.
func NewProjectMetadata(rootdir string, stacks List[*Stack]) project.Metadata {
	stackPaths := make(project.Paths, len(stacks))
	for i, st := range stacks {
		stackPaths[i] = st.Dir()
	}
	return project.NewMetadata(rootdir, stackPaths)
}

// LoadAllStacks loads all stacks inside the given rootdir.
func LoadAllStacks(cfg *Tree) (List[*Stack], error) {
	logger := log.With().
		Str("action", "stack.LoadAll()").
		Str("root", cfg.RootDir()).
		Logger()

	stacks := List[*Stack]{}
	stacksIDs := map[string]*Stack{}

	for _, stackNode := range cfg.Stacks() {
		stack, err := NewStack(cfg.RootDir(), stackNode.Node)
		if err != nil {
			return List[*Stack]{}, err
		}

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Debug().Msg("Found stack")
		stacks = append(stacks, stack)

		if id, ok := stack.ID(); ok {
			logger.Trace().Msg("stack has ID, checking for duplicate")
			if otherStack, ok := stacksIDs[id]; ok {
				return List[*Stack]{}, errors.E(ErrStackDuplicatedID,
					"stack %q and %q have same ID %q",
					stack.Dir(),
					otherStack.Dir(),
					id,
				)
			}
			stacksIDs[id] = stack
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
	return NewStack(root.HostDir(), node.Node)
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

	s, err := NewStack(root.HostDir(), tree.Node)
	if err != nil {
		return nil, true, err
	}
	return s, true, nil
}

// ReverseStacks reverses the given stacks slice.
func ReverseStacks(stacks List[*Stack]) {
	i, j := 0, len(stacks)-1
	for i < j {
		stacks[i], stacks[j] = stacks[j], stacks[i]
		i++
		j--
	}
}

// Paths returns the project paths from the stack list.
func (l List[T]) Paths() project.Paths {
	paths := make(project.Paths, len(l))
	for i, s := range l {
		paths[i] = s.Dir()
	}
	return paths
}

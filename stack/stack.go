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

package stack

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
)

type (
	// S represents a stack
	S struct {
		// hostpath is the file system absolute path of the stack.
		hostpath string

		// path is the absolute path of the stack relative to project's root.
		path project.Path

		// relPathToRoot is the relative path from the stack to root.
		relPathToRoot string

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

	// Metadata has all metadata loaded per stack
	Metadata interface {
		// ID of the stack if it has any. Empty string and false otherwise.
		ID() (string, bool)
		// Name of the stack.
		Name() string
		// HostPath is the absolute path of the stack on the host file system.
		HostPath() string
		// Path is the absolute path of the stack (relative to project root).
		Path() project.Path
		// RelPath is the relative path of the from root.
		RelPath() string
		// PathBase is the basename of the stack path.
		PathBase() string
		// Desc is the description of the stack (relative to project root).
		Desc() string
		// RelPathToRoot is the relative path from the stack to root.
		RelPathToRoot() string
	}

	// List of stacks.
	List []*S
)

const (
	// ErrDuplicatedID indicates that two or more stacks have the same ID.
	ErrDuplicatedID errors.Kind = "duplicated ID found on stacks"

	// ErrInvalidWatch indicates the stack.watch attribute contains invalid values.
	ErrInvalidWatch errors.Kind = "invalid stack.watch attribute"
)

// New creates a new stack from configuration cfg.
func New(root string, cfg hcl.Config) (*S, error) {
	name := cfg.Stack.Name
	if name == "" {
		name = filepath.Base(cfg.AbsDir())
	}

	rel, err := filepath.Rel(cfg.AbsDir(), root)
	if err != nil {
		// This is an invariant on Terramate, stacks must always be
		// inside the root dir.
		panic(errors.E(
			"No relative path from stack %q to root %q",
			cfg.AbsDir(), root, err))
	}

	watchFiles, err := validateWatchPaths(root, cfg.AbsDir(), cfg.Stack.Watch)
	if err != nil {
		return nil, errors.E(err, ErrInvalidWatch)
	}

	return &S{
		name:          name,
		id:            cfg.Stack.ID,
		desc:          cfg.Stack.Description,
		after:         cfg.Stack.After,
		before:        cfg.Stack.Before,
		wants:         cfg.Stack.Wants,
		wantedBy:      cfg.Stack.WantedBy,
		watch:         watchFiles,
		hostpath:      cfg.AbsDir(),
		path:          project.PrjAbsPath(root, cfg.AbsDir()),
		relPathToRoot: filepath.ToSlash(rel),
	}, nil
}

// ID of the stack if it has one, or empty string and false otherwise.
func (s *S) ID() (string, bool) {
	return s.id.Value()
}

// Name of the stack.
func (s *S) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.Path().String()
}

// Desc is the description of the stack.
func (s *S) Desc() string { return s.desc }

// After specifies the list of stacks that must run before this stack.
func (s S) After() []string { return s.after }

// Before specifies the list of stacks that must run after this stack.
func (s S) Before() []string { return s.before }

// AppendBefore appends the path to the list of stacks that must run after this
// stack.
func (s *S) AppendBefore(path string) {
	s.before = append(s.before, path)
}

// Wants specifies the list of wanted stacks.
func (s S) Wants() []string { return s.wants }

// WantedBy specifies the list of stacks that wants this stack.
func (s S) WantedBy() []string { return s.wantedBy }

// Watch returns the list of watched files.
func (s *S) Watch() []project.Path { return s.watch }

// IsChanged tells if the stack is marked as changed.
func (s *S) IsChanged() bool { return s.changed }

// SetChanged sets the changed flag of the stack.
func (s *S) SetChanged(b bool) { s.changed = b }

// String representation of the stack.
func (s *S) String() string { return s.Path().String() }

// Path returns the project's absolute path of stack.
func (s *S) Path() project.Path { return s.path }

// PathBase returns the base name of the stack path.
func (s *S) PathBase() string { return filepath.Base(s.path.String()) }

// RelPath returns the project's relative path of stack.
func (s *S) RelPath() string { return s.path.String()[1:] }

// RelPathToRoot returns the relative path from the stack to root.
func (s *S) RelPathToRoot() string { return s.relPathToRoot }

// HostPath returns the file system absolute path of stack.
func (s *S) HostPath() string { return s.hostpath }

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

// TreeListToStackList converts a config.List into a stack.List.
// TODO(i4k): improve this naming.
func TreeListToStackList(root string, trees config.List) (List, error) {
	var stacks List
	for _, tree := range trees {
		s, err := New(root, tree.Node)
		if err != nil {
			return List{}, err
		}
		stacks = append(stacks, s)
	}
	return stacks, nil
}

// NewProjectMetadata creates project metadata from a given rootdir and a list of stacks.
func NewProjectMetadata(rootdir string, stacks List) project.Metadata {
	stackPaths := make(project.Paths, len(stacks))
	for i, st := range stacks {
		stackPaths[i] = st.Path()
	}
	return project.NewMetadata(rootdir, stackPaths)
}

// LoadAll loads all stacks inside the given rootdir.
func LoadAll(cfg *config.Tree) (List, error) {
	logger := log.With().
		Str("action", "stack.LoadAll()").
		Str("root", cfg.RootDir()).
		Logger()

	stacks := List{}
	stacksIDs := map[string]*S{}

	for _, stackNode := range cfg.Stacks() {
		stack, err := New(cfg.RootDir(), stackNode.Node)
		if err != nil {
			return List{}, err
		}

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Debug().Msg("Found stack")
		stacks = append(stacks, stack)

		if id, ok := stack.ID(); ok {
			logger.Trace().Msg("stack has ID, checking for duplicate")
			if otherStack, ok := stacksIDs[id]; ok {
				return List{}, errors.E(ErrDuplicatedID,
					"stack %q and %q have same ID %q",
					stack.Path(),
					otherStack.Path(),
					id,
				)
			}
			stacksIDs[id] = stack
		}
	}

	return stacks, nil
}

// Load a single stack from dir.
func Load(cfg *config.Tree, dir string) (*S, error) {
	path := project.PrjAbsPath(cfg.RootDir(), dir)
	node, ok := cfg.Lookup(path)
	if !ok {
		return nil, errors.E("config not found at %s", path)
	}
	if !node.IsStack() {
		return nil, errors.E("config at %s is not a stack")
	}
	return New(cfg.RootDir(), node.Node)
}

// TryLoad tries to load a single stack from dir. It sets found as true in case
// the stack was successfully loaded.
func TryLoad(root, absdir string) (stack *S, found bool, err error) {
	logger := log.With().
		Str("action", "TryLoad()").
		Str("dir", absdir).
		Logger()

	if !strings.HasPrefix(absdir, root) {
		return nil, false, errors.E(fmt.Sprintf("directory %s is not inside project root %s",
			absdir, root))
	}

	logger.Debug().Msg("Parsing configuration.")
	cfg, err := hcl.ParseDir(root, absdir)
	if err != nil {
		return nil, false, errors.E(err, "failed to parse directory %s", absdir)
	}

	if cfg.Stack == nil {
		return nil, false, nil
	}

	logger.Debug().Msg("Create a new stack")
	s, err := New(root, cfg)
	if err != nil {
		return nil, true, err
	}
	return s, true, nil
}

// Sort sorts the given stacks.
func Sort(stacks []*S) {
	sort.Sort(List(stacks))
}

// Reverse reverses the given stacks slice.
func Reverse(stacks List) {
	i, j := 0, len(stacks)-1
	for i < j {
		stacks[i], stacks[j] = stacks[j], stacks[i]
		i++
		j--
	}
}

// Paths returns the project paths from the stack list.
func (l List) Paths() project.Paths {
	paths := make(project.Paths, len(l))
	for i, s := range l {
		paths[i] = s.Path()
	}
	return paths
}

func (l List) Len() int           { return len(l) }
func (l List) Less(i, j int) bool { return l[i].Path() < l[j].Path() }
func (l List) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

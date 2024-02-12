// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
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
	// ErrStackValidation indicates an error when validating the stack fields.
	ErrStackValidation errors.Kind = "validating stack fields"

	// ErrStackDuplicatedID indicates that two or more stacks have the same ID.
	ErrStackDuplicatedID errors.Kind = "duplicated ID found on stacks"

	// ErrStackInvalidWatch indicates the stack.watch attribute contains invalid values.
	ErrStackInvalidWatch errors.Kind = "invalid stack.watch attribute"

	// ErrStackInvalidTag indicates the stack.tags is invalid.
	ErrStackInvalidTag errors.Kind = "invalid stack.tags entry"

	// ErrStackInvalidWants indicates the stack.wants is invalid.
	ErrStackInvalidWants errors.Kind = "invalid stack.wants entry"

	// ErrStackInvalidWantedBy indicates the stack.wanted_by is invalid.
	ErrStackInvalidWantedBy errors.Kind = "invalid stack.wanted_by entry"
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
	errs := errors.L()
	errs.AppendWrap(ErrStackValidation, s.validateID(), s.ValidateSets(), s.ValidateTags())
	return errs.AsError()
}

// ValidateTags validates if tags are correctly used in all stack fields.
func (s Stack) ValidateTags() error {
	errs := errors.L()
	errs.Append(s.validateTagsField())
	errs.AppendWrap(ErrStackInvalidWants, s.validateTagFilterNotAllowed(s.Wants))
	errs.AppendWrap(ErrStackInvalidWantedBy, s.validateTagFilterNotAllowed(s.WantedBy))
	return errs.AsError()
}

func (s Stack) validateTagsField() error {
	for _, tagname := range s.Tags {
		err := tag.Validate(tagname)
		if err != nil {
			return errors.E(ErrStackInvalidTag, err)
		}
	}
	return nil
}

const stackIDRegexPattern = "^[a-zA-Z0-9_-]{1,64}$"

var _ = regexp.MustCompile(stackIDRegexPattern)

func (s Stack) validateID() error {
	if s.ID == "" {
		return nil
	}
	stackIDRegex := regexp.MustCompile(stackIDRegexPattern)
	if !stackIDRegex.MatchString(s.ID) {
		return errors.E("stack ID %q doesn't match %q", s.ID, stackIDRegexPattern)
	}
	return nil
}

// ValidateSets validate all stack set fields.
func (s Stack) ValidateSets() error {
	errs := errors.L(
		validateSet("tags", s.Tags),
		validateSet("after", s.After),
		validateSet("before", s.Before),
		validateSet("wants", s.Wants),
		validateSet("wanted_by", s.WantedBy),
	)
	return errs.AsError()
}

func validateSet(field string, set []string) error {
	elems := map[string]struct{}{}
	for _, s := range set {
		if _, ok := elems[s]; ok {
			return errors.E("field %s has duplicate %q element", field, s)
		}
		elems[s] = struct{}{}
	}
	return nil
}
func (s Stack) validateTagFilterNotAllowed(cfglst ...[]string) error {
	for _, lst := range cfglst {
		for _, elem := range lst {
			if strings.HasPrefix(elem, "tag:") {
				return errors.E("tag:<query> filter is not allowed")
			}
		}
	}
	return nil
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
func StacksFromTrees(trees List[*Tree]) (List[*SortableStack], error) {
	var stacks List[*SortableStack]
	for _, tree := range trees {
		s, err := tree.Stack()
		if err != nil {
			return nil, err
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
		stack, err := stackNode.Stack()
		if err != nil {
			return nil, err
		}

		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Debug().Msg("Found stack")
		stacks = append(stacks, stack.Sortable())

		if stack.ID != "" {
			if otherStack, ok := stacksIDs[strings.ToLower(stack.ID)]; ok {
				return List[*SortableStack]{}, errors.E(ErrStackDuplicatedID,
					"stack %q and %q have same ID %q",
					stack.Dir,
					otherStack.Dir,
					stack.ID,
				)
			}
			stacksIDs[strings.ToLower(stack.ID)] = stack
		}
	}

	return stacks, nil
}

// LoadStack a single stack from dir.
func LoadStack(root *Root, dir project.Path) (*Stack, error) {
	node, ok := root.Lookup(dir)
	if !ok {
		return nil, errors.E("config not found at %q", dir)
	}
	if !node.IsStack() {
		return nil, errors.E("config at %q is not a stack", dir)
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

	s, err := tree.Stack()
	return s, true, err
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

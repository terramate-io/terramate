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
	"sort"
	"strings"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config/filter"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// DefaultFilename is the name of the default Terramate configuration file.
	DefaultFilename = "terramate.tm.hcl"

	// SkipFilename is the name of Terramate skip file.
	SkipFilename = ".tmskip"
)

const (
	// ErrSchema indicates that the configuration has an invalid schema.
	ErrSchema errors.Kind = "config has an invalid schema"
)

// Root is the root configuration tree.
// This type is just for ensure better type checking for the cases where a
// configuration for the root directory is expected and not from anywhere else.
type Root struct {
	tree Tree

	runtime project.Runtime
}

// Tree is the configuration tree.
// The tree maps the filesystem directories, which means each directory in the
// project has a tree instance even if it's empty (ie no .tm files in it).
type Tree struct {
	// Node is the configuration of this tree node.
	Node hcl.Config

	// Children is a map of configuration dir names to tree nodes.
	Children map[string]*Tree

	// Parent is the parent node or nil if none.
	Parent *Tree

	dir string
}

// DirElem represents a node which is represented by a directory.
// Eg.: stack, config, etc.
type DirElem interface {
	Dir() project.Path
}

// List of directory based elements which implements the sorting interface
// by the directory path.
type List[T DirElem] []T

// TryLoadConfig try to load the Terramate configuration tree. It looks for the
// the config in fromdir and all parent directories until / is reached.
// If the configuration is found, it returns the whole configuration tree,
// configpath != "" and found as true.
func TryLoadConfig(fromdir string) (tree *Root, configpath string, found bool, err error) {
	for {
		logger := log.With().
			Str("action", "config.TryLoadConfig()").
			Str("path", fromdir).
			Logger()

		logger.Trace().Msg("Parse Terramate config.")

		cfg, err := hcl.ParseDir(fromdir, fromdir)
		if err != nil {
			// the imports only works for the correct rootdir.
			// As we are looking for the rootdir, we should ignore ErrImport
			// errors.
			if !errors.IsKind(err, hcl.ErrImport) {
				return nil, "", false, err
			}
		} else if cfg.Terramate != nil && cfg.Terramate.Config != nil {
			tree, err := loadTree(fromdir, fromdir, &cfg)
			if err != nil {
				return nil, fromdir, true, err
			}
			return NewRoot(tree), fromdir, true, err
		}

		parent, ok := parentDir(fromdir)
		if !ok {
			break
		}
		fromdir = parent
	}
	return nil, "", false, nil
}

// NewRoot creates a new [Root] tree for the cfg tree.
func NewRoot(tree *Tree) *Root {
	r := &Root{
		tree: *tree,
	}
	r.initRuntime()
	return r
}

// LoadRoot loads the root configuration tree.
func LoadRoot(rootdir string) (*Root, error) {
	cfgtree, err := LoadTree(rootdir, rootdir)
	if err != nil {
		return nil, err
	}
	return NewRoot(cfgtree), nil
}

// Tree returns the root configuration tree.
func (root *Root) Tree() *Tree { return &root.tree }

// HostDir returns the root directory.
func (root *Root) HostDir() string { return root.tree.RootDir() }

// Lookup a node from the root using a filesystem query path.
func (root *Root) Lookup(path project.Path) (*Tree, bool) {
	return root.tree.lookup(path)
}

// StacksByPaths returns the stacks from the provided relative paths.
func (root *Root) StacksByPaths(base project.Path, relpaths ...string) List[*Tree] {
	logger := log.With().
		Str("action", "root.StacksByPath").
		Stringer("basedir", base).
		Strs("paths", relpaths).
		Logger()

	logger.Trace().Msg("lookup paths")

	normalizePaths := func(paths []string) []project.Path {
		pathmap := map[string]struct{}{}
		var normalized []project.Path
		for _, p := range paths {
			var pathstr string
			if path.IsAbs(p) {
				pathstr = p
			} else {
				pathstr = path.Join(base.String(), p)
			}
			if _, ok := pathmap[pathstr]; !ok {
				pathmap[pathstr] = struct{}{}
				normalized = append(normalized, project.NewPath(pathstr))
			}
		}
		return normalized
	}

	var stacks List[*Tree]
	for _, path := range normalizePaths(relpaths) {
		node, ok := root.Lookup(path)
		if !ok {
			logger.Warn().Msgf("path %s not found in configuration", path.String())
			continue
		}
		stacks = append(stacks, node.stacks((*Tree).IsStack)...)
	}

	sort.Sort(stacks)

	logger.Trace().Msgf("found %d stacks out of %d paths", len(stacks), len(relpaths))

	return stacks
}

// StacksByTagsFilters returns the paths of all stacks matching the filters.
func (root *Root) StacksByTagsFilters(filters []string) (project.Paths, error) {
	clauses, hasFilter, err := filter.ParseTagClauses(filters...)
	if err != nil {
		return nil, err
	}
	return root.tree.stacks(func(tree *Tree) bool {
		if !hasFilter || !tree.IsStack() {
			return false
		}
		return filter.MatchTags(clauses, tree.Node.Stack.Tags)
	}).Paths(), nil
}

// LoadSubTree loads a subtree located at cfgdir into the current tree.
func (root *Root) LoadSubTree(cfgdir project.Path) error {
	var parent project.Path

	var parentNode *Tree
	parent = cfgdir.Dir()
	for parent.String() != "/" {
		var found bool
		parentNode, found = root.Lookup(parent)
		if found {
			break
		}
		parent = parent.Dir()
	}

	if parentNode == nil {
		parentNode = root.Tree()
	}

	rootdir := root.HostDir()

	relpath := strings.TrimPrefix(cfgdir.String(), parent.String())
	relpath = strings.TrimPrefix(relpath, "/")
	components := strings.Split(relpath, "/")
	nextComponent := components[0]
	subtreeDir := filepath.Join(rootdir, parent.String(), nextComponent)

	node, err := LoadTree(rootdir, subtreeDir)
	if err != nil {
		return errors.E(err, "failed to load config from %s", subtreeDir)
	}

	if node.HostDir() == rootdir {
		// root configuration reloaded
		*root = *NewRoot(node)
	} else {
		node.Parent = parentNode
		parentNode.Children[nextComponent] = node
	}
	return nil
}

// Stacks return the stacks paths.
func (root *Root) Stacks() project.Paths {
	return root.tree.Stacks().Paths()
}

// Runtime returns a copy the runtime for the root terramate namespace as a
// cty.Value map.
func (root *Root) Runtime() project.Runtime {
	runtime := project.Runtime{}
	for k, v := range root.runtime {
		runtime[k] = v
	}
	return runtime
}

func (root *Root) initRuntime() {
	rootfs := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(root.HostDir()),
		"basename": cty.StringVal(filepath.Base(root.HostDir())),
	})
	rootpath := cty.ObjectVal(map[string]cty.Value{
		"fs": rootfs,
	})
	rootNS := cty.ObjectVal(map[string]cty.Value{
		"path": rootpath,
	})
	stacksNs := cty.ObjectVal(map[string]cty.Value{
		"list": toCtyStringList(root.Stacks().Strings()),
	})
	root.runtime = project.Runtime{
		"root":    rootNS,
		"stacks":  stacksNs,
		"version": cty.StringVal(terramate.Version()),
	}
}

// LoadTree loads the whole hierarchical configuration from cfgdir downwards
// using rootdir as project root.
func LoadTree(rootdir string, cfgdir string) (*Tree, error) {
	return loadTree(rootdir, cfgdir, nil)
}

// HostDir is the node absolute directory in the host.
func (tree *Tree) HostDir() string {
	return tree.dir
}

// Dir returns the directory as a project dir.
func (tree *Tree) Dir() project.Path {
	return project.PrjAbsPath(tree.RootDir(), tree.dir)
}

// RootDir returns the tree root directory..
func (tree *Tree) RootDir() string {
	if tree.Parent != nil {
		return tree.Parent.RootDir()
	}
	return tree.dir
}

// Root returns the root of the configuration tree.
func (tree *Tree) Root() *Root {
	if tree.Parent != nil {
		return tree.Parent.Root()
	}
	return NewRoot(tree)
}

// IsStack tells if the node is a stack.
func (tree *Tree) IsStack() bool {
	return tree.Node.Stack != nil
}

// Stacks returns the stack nodes from the tree.
// The search algorithm is a Deep-First-Search (DFS).
func (tree *Tree) Stacks() List[*Tree] {
	stacks := tree.stacks((*Tree).IsStack)
	sort.Sort(stacks)
	return stacks
}

func (tree *Tree) stacks(cond func(*Tree) bool) List[*Tree] {
	var stacks List[*Tree]
	if cond(tree) {
		stacks = append(stacks, tree)
	}
	for _, children := range tree.Children {
		stacks = append(stacks, children.stacks(cond)...)
	}
	return stacks
}

// Lookup a node from the tree using a filesystem query path.
// The abspath is relative to the current tree node.
func (tree *Tree) lookup(abspath project.Path) (*Tree, bool) {
	pathstr := abspath.String()
	if len(pathstr) == 0 || pathstr[0] != '/' {
		return nil, false
	}

	parts := strings.Split(pathstr, "/")
	cfg := tree
	parts = parts[1:] // skip root/current cfg
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		node, found := cfg.Children[parts[i]]
		if !found {
			return nil, false
		}
		cfg = node
	}
	return cfg, true
}

// AsList returns a list with this node and all its children.
func (tree *Tree) AsList() List[*Tree] {
	result := List[*Tree]{
		tree,
	}

	for _, children := range tree.Children {
		result = append(result, children.AsList()...)
	}
	return result
}

func (l List[T]) Len() int           { return len(l) }
func (l List[T]) Less(i, j int) bool { return l[i].Dir().String() < l[j].Dir().String() }
func (l List[T]) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func loadTree(rootdir string, cfgdir string, rootcfg *hcl.Config) (_ *Tree, err error) {
	logger := log.With().
		Str("action", "config.loadTree()").
		Str("dir", rootdir).
		Logger()

	f, err := os.Open(cfgdir)
	if err != nil {
		return nil, errors.E(err, "failed to open cfg directory")
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	logger.Trace().Msg("reading directory file names")

	names, err := f.Readdirnames(0)
	if err != nil {
		return nil, errors.E(err, "failed to read files in %s", cfgdir)
	}

	for _, name := range names {
		if name == SkipFilename {
			logger.Debug().Msg("skip file found: skipping whole subtree")
			return NewTree(cfgdir), nil
		}
	}

	tree := NewTree(cfgdir)
	if rootcfg != nil {
		tree.Node = *rootcfg
	} else {
		cfg, err := hcl.ParseDir(rootdir, cfgdir)
		if err != nil {
			return nil, err
		}
		tree.Node = cfg
	}

	for _, name := range names {
		logger = logger.With().
			Str("filename", name).
			Logger()

		if Skip(name) {
			logger.Trace().Msg("skipping file")
			continue
		}
		dir := filepath.Join(cfgdir, name)
		st, err := os.Lstat(dir)
		if err != nil {
			return nil, errors.E(err, "failed to stat %s", dir)
		}
		if !st.IsDir() {
			logger.Trace().Msg("ignoring non-directory file")
			continue
		}

		logger.Trace().Msg("loading children tree")

		node, err := LoadTree(rootdir, dir)
		if err != nil {
			return nil, errors.E(err, "loading from %s", dir)
		}

		node.Parent = tree
		tree.Children[name] = node
	}
	return tree, nil
}

// IsEmptyConfig tells if the configuration is empty.
func (tree *Tree) IsEmptyConfig() bool {
	return tree.Node.IsEmpty()
}

// NonEmptyGlobalsParent returns a parent configuration which has globals defined, if any.
func (tree *Tree) NonEmptyGlobalsParent() *Tree {
	parent := tree.Parent
	for parent != nil && !parent.Node.HasGlobals() {
		parent = parent.Parent
	}
	return parent
}

// IsStack returns true if the given directory is a stack, false otherwise.
func IsStack(root *Root, dir string) bool {
	node, ok := root.Lookup(project.PrjAbsPath(root.HostDir(), dir))
	return ok && node.IsStack()
}

// NewTree creates a new tree node.
func NewTree(cfgdir string) *Tree {
	return &Tree{
		dir:      cfgdir,
		Children: make(map[string]*Tree),
	}
}

// Skip returns true if the given file/dir name should be ignored by Terramate.
func Skip(name string) bool {
	// assumes filename length > 0
	return name[0] == '.'
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}

func toCtyStringList(list []string) cty.Value {
	if len(list) == 0 {
		// cty panics if the list is empty
		return cty.ListValEmpty(cty.String)
	}
	res := make([]cty.Value, len(list))
	for i, elem := range list {
		res[i] = cty.StringVal(elem)
	}
	return cty.ListVal(res)
}

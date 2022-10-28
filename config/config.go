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
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
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

// List of config trees.
type List []*Tree

// TryLoadConfig try to load the Terramate configuration tree. It looks for the
// the config in fromdir and all parent directories until / is reached.
// If the configuration is found, it returns the whole configuration tree,
// configpath != "" and found as true.
func TryLoadConfig(fromdir string) (
	tree *Tree, configpath string, found bool, isgit bool, err error,
) {
	for {
		p, err := hcl.NewTerramateParser(fromdir, fromdir)
		if err != nil {
			return nil, "", false, false, err
		}

		isgitroot := false
		names, err := fs.ListTerramateFiles(fromdir, func(name string) {
			if name == ".git" {
				isgitroot = true
			}
		})

		if err != nil {
			return nil, "", false, false, err
		}

		for _, name := range names {
			dir := filepath.Join(fromdir, name)
			err = p.AddFile(dir)
			if err != nil {
				return nil, "", false, false, err
			}
		}

		cfg, err := p.ParseConfig()
		if err != nil {
			// the imports only works for the correct rootdir.
			// As we are looking for the rootdir, we should ignore ErrImport
			// errors.
			if !errors.IsKind(err, hcl.ErrImport) {
				return nil, "", false, false, err
			}
		} else if isgitroot || (cfg.Terramate != nil && cfg.Terramate.Config != nil) {
			tree, err := loadTree(fromdir, fromdir, &cfg)
			return tree, fromdir, true, isgitroot, err
		}

		parent, ok := parentDir(fromdir)
		if !ok {
			break
		}
		fromdir = parent
	}
	return nil, "", false, false, nil
}

// LoadTree loads the whole hierarchical configuration from cfgdir downwards
// using rootdir as project root.
func LoadTree(rootdir string, cfgdir string) (*Tree, error) {
	return loadTree(rootdir, cfgdir, nil)
}

// Dir is the node directory.
func (tree *Tree) Dir() string {
	return tree.dir
}

// RootDir returns the tree root directory..
func (tree *Tree) RootDir() string {
	if tree.Parent != nil {
		return tree.Parent.RootDir()
	}
	return tree.dir
}

// Root returns the root of the configuration tree.
func (tree *Tree) Root() *Tree {
	if tree.Parent != nil {
		return tree.Parent.Root()
	}
	return tree
}

// IsStack tells if the node is a stack.
func (tree *Tree) IsStack() bool {
	return tree.Node.Stack != nil
}

// Stacks returns the stack nodes from the tree.
// The search algorithm is a Deep-First-Search (DFS).
func (tree *Tree) Stacks() List {
	stacks := tree.stacks()
	sort.Sort(stacks)
	return stacks
}

func (tree *Tree) stacks() List {
	var stacks List
	if tree.IsStack() {
		stacks = append(stacks, tree)
	}
	for _, children := range tree.Children {
		stacks = append(stacks, children.stacks()...)
	}
	return stacks
}

// Lookup a node from the tree using a filesystem query path.
// The abspath is relative to the current tree node.
func (tree *Tree) Lookup(abspath project.Path) (*Tree, bool) {
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

// StacksByPaths returns the stacks from the provided relative paths.
func (tree *Tree) StacksByPaths(base project.Path, relpaths ...string) List {
	logger := log.With().
		Str("action", "tree.StacksByPath").
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

	var stacks List
	for _, path := range normalizePaths(relpaths) {
		node, ok := tree.Lookup(path)
		if !ok {
			logger.Warn().Msgf("path %s not found in configuration", path.String())
			continue
		}
		stacks = append(stacks, node.stacks()...)
	}

	sort.Sort(stacks)

	logger.Trace().Msgf("found %d stacks out of %d paths", len(stacks), len(relpaths))

	return stacks
}

func (l List) Len() int           { return len(l) }
func (l List) Less(i, j int) bool { return l[i].dir < l[j].dir }
func (l List) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func loadTree(rootdir string, cfgdir string, rootcfg *hcl.Config) (*Tree, error) {
	var skipdir bool
	names, err := fs.ListTerramateDirs(cfgdir, func(name string) {
		if !skipdir {
			skipdir = name == SkipFilename
		}
	})

	if err != nil {
		return nil, err
	}

	tree := NewTree(cfgdir)
	if skipdir {
		return tree, nil
	}

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
		dir := filepath.Join(cfgdir, name)
		node, err := LoadTree(rootdir, dir)
		if err != nil {
			return nil, errors.E(err, "loading from %s", dir)
		}

		node.Parent = tree
		tree.Children[name] = node
	}
	return tree, nil
}

// LoadSubTree loads a subtree located at cfgdir into the current tree.
func (tree *Tree) LoadSubTree(cfgdir project.Path) error {
	var parent project.Path

	var parentNode *Tree
	parent = cfgdir.Dir()
	for parent != "/" {
		var found bool
		parentNode, found = tree.Lookup(parent)
		if found {
			break
		}
		parent = parent.Dir()
	}

	if parentNode == nil {
		parentNode = tree
	}

	rootdir := tree.RootDir()

	relpath := strings.TrimPrefix(cfgdir.String(), parent.String())
	relpath = strings.TrimPrefix(relpath, "/")
	components := strings.Split(relpath, "/")
	nextComponent := components[0]
	subtreeDir := filepath.Join(rootdir, parent.String(), nextComponent)

	node, err := LoadTree(rootdir, subtreeDir)
	if err != nil {
		return errors.E(err, "failed to load config from %s", subtreeDir)
	}

	if node.Dir() == rootdir {
		// root configuration reloaded
		*tree = *node
	} else {
		node.Parent = parentNode
		parentNode.Children[nextComponent] = node
	}
	return nil
}

// IsEmptyConfig tells if the configuration is empty.
func (tree *Tree) IsEmptyConfig() bool {
	return tree.Node.IsEmpty()
}

// IsStack returns true if the given directory is a stack, false otherwise.
func IsStack(cfg *Tree, dir string) bool {
	node, ok := cfg.Lookup(project.PrjAbsPath(cfg.RootDir(), dir))
	return ok && node.IsStack()
}

// NewTree creates a new tree node.
func NewTree(cfgdir string) *Tree {
	return &Tree{
		dir:      cfgdir,
		Children: make(map[string]*Tree),
	}
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}

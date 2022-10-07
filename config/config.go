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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultFilename is the name of the default Terramate configuration file.
	DefaultFilename = "terramate.tm.hcl"
)

const (
	// ErrSchema indicates that the configuration has an invalid schema.
	ErrSchema errors.Kind = "config has an invalid schema"
)

// Tree is the config tree.
type Tree struct {
	// Root is the configuration of this tree node.
	Root hcl.Config

	// Children is a map of configuration dir names to tree nodes.
	Children map[string]*Tree

	// Parent is the parent node or nil if none.
	Parent *Tree

	rootdir string
}

type List []*Tree

// TryLoadConfig try to load the Terramate configuration tree. It looks for the
// the config in fromdir and all parent directories until / is reached.
// If the configuration is found, it returns configpath != "" and found as true.
func TryLoadConfig(fromdir string) (tree *Tree, configpath string, found bool, err error) {
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
			return tree, fromdir, true, err
		}

		parent, ok := parentDir(fromdir)
		if !ok {
			break
		}
		fromdir = parent
	}
	return nil, "", false, nil
}

func LoadTree(rootdir string, cfgdir string) (*Tree, error) {
	return loadTree(rootdir, cfgdir, nil)
}

func (tree *Tree) Rootdir() string {
	return tree.rootdir
}

func (tree *Tree) IsStack() bool {
	return tree.Root.Stack != nil
}

func (tree *Tree) Stacks() []*Tree {
	var stacks List
	if tree.IsStack() {
		stacks = append(stacks, tree)
	}
	for _, children := range tree.Children {
		stacks = append(stacks, children.Stacks()...)
	}

	stacks.Sort()

	return stacks
}

func (tree *Tree) Lookup(path project.Path) (*Tree, bool) {
	pathstr := path.String()
	if len(pathstr) == 0 || pathstr[0] != '/' {
		return nil, false
	}

	parts := strings.Split(pathstr, "/")
	cfg := tree
	parts = parts[1:] // ignore root cfg
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

func (tree *Tree) StacksByRelPaths(base project.Path, paths ...string) List {
	var stacks List
	for _, p := range paths {
		pathstr := path.Join(base.String(), p)
		node, ok := tree.Lookup(project.NewPath(pathstr))
		if !ok {
			continue
		}
		stacks = append(stacks, node.Stacks()...)
	}
	stacks.Sort()
	return stacks
}

func (l List) Sort() {
	sort.Slice(l, func(i, j int) bool {
		return l[i].Rootdir() < l[j].Rootdir()
	})
}

func loadTree(rootdir string, cfgdir string, rootcfg *hcl.Config) (*Tree, error) {
	logger := log.With().
		Str("action", "config.LoadTree()").
		Str("dir", rootdir).
		Logger()

	tree := NewTree(cfgdir)
	if rootcfg != nil {
		tree.Root = *rootcfg
	} else {
		cfg, err := hcl.ParseDir(rootdir, cfgdir)
		if err != nil {
			return nil, err
		}
		tree.Root = cfg
	}

	f, err := os.Open(cfgdir)
	if err != nil {
		return nil, errors.E(err, "failed to open rootdir directory")
	}

	logger.Trace().Msg("reading directory file names")

	names, err := f.Readdirnames(0)
	if err != nil {
		return nil, errors.E(err, "failed to read files in %s", rootdir)
	}
	for _, name := range names {
		logger = logger.With().
			Str("filename", name).
			Logger()

		if ignoreFilename(name) {
			logger.Trace().Msg("ignoring dot file")
			continue
		}
		fname := filepath.Join(cfgdir, name)
		st, err := os.Lstat(fname)
		if err != nil {
			return nil, errors.E(err, "failed to stat %s", fname)
		}
		if !st.IsDir() {
			logger.Trace().Msg("ignoring non-directory file")
			continue
		}

		logger.Trace().Msg("loading children tree")

		node, err := LoadTree(rootdir, fname)
		if err != nil {
			return nil, errors.E(err, "failed to load config from %s", fname)
		}

		node.Parent = tree
		tree.Children[name] = node
	}
	return tree, nil
}

func (tree *Tree) IsEmptyConfig() bool {
	return tree.Root.IsEmpty()
}

// IsStack returns true if the given directory is a stack, false otherwise.
func IsStack(cfg *Tree, dir string) bool {
	node, ok := cfg.Lookup(project.PrjAbsPath(cfg.Rootdir(), dir))
	return ok && node.IsStack()
}

func NewTree(cfgdir string) *Tree {
	return &Tree{
		rootdir:  cfgdir,
		Children: make(map[string]*Tree),
	}
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}

func ignoreFilename(name string) bool {
	return name[0] == '.' // assumes filename length > 0
}

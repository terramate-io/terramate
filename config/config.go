// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/config/filter"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
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

	lookupCache map[string]*Tree

	// hasTerragruntStacks tells if the repository has any Terragrunt stack.
	hasTerragruntStacks *bool
	runtime             project.Runtime
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

	// project root is only set if Parent == nil.
	root  *Root
	stack *Stack

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
		ok, err := hcl.IsRootConfig(fromdir)
		if err != nil {
			return nil, "", false, err
		}

		if ok {
			cfg, err := hcl.ParseDir(fromdir, fromdir)
			if err != nil {
				return nil, fromdir, true, err
			}
			rootTree := NewTree(fromdir)
			rootTree.Node = cfg
			_, err = loadTree(rootTree, fromdir, nil)
			if err != nil {
				return nil, fromdir, true, err
			}
			return NewRoot(rootTree), fromdir, true, err
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
	r := &Root{}
	tree.root = r
	r.tree = *tree
	r.lookupCache = make(map[string]*Tree)

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
	tree, ok := root.lookupCache[path.String()]
	if ok {
		return tree, tree != nil
	}
	tree, ok = root.tree.lookup(path)
	root.lookupCache[path.String()] = tree
	return tree, ok
}

// StacksByPaths returns the stacks from the provided relative paths.
func (root *Root) StacksByPaths(base project.Path, relpaths ...string) List[*Tree] {
	logger := log.With().
		Str("action", "root.StacksByPath").
		Stringer("basedir", base).
		Strs("paths", relpaths).
		Logger()

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
	cfg, err := hcl.ParseDir(rootdir, rootdir)
	if err != nil {
		return nil, err
	}
	root := NewTree(rootdir)
	root.Node = cfg
	return loadTree(root, cfgdir, nil)
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
	return tree.root
}

// RootTree returns the tree at the project root.
func (tree *Tree) RootTree() *Tree {
	if tree.Parent != nil {
		return tree.Parent.RootTree()
	}
	return tree
}

// IsStack tells if the node is a stack.
func (tree *Tree) IsStack() bool {
	return tree.Node.Stack != nil
}

// Stack returns the stack object.
func (tree *Tree) Stack() (*Stack, error) {
	if tree.stack == nil {
		s, err := LoadStack(tree.Root(), tree.Dir())
		if err != nil {
			return nil, err
		}
		tree.stack = s
	}
	return tree.stack, nil
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

func loadTree(parentTree *Tree, cfgdir string, rootcfg *hcl.Config) (_ *Tree, err error) {
	logger := log.With().
		Str("action", "config.loadTree()").
		Str("dir", cfgdir).
		Logger()

	f, err := os.Open(cfgdir)
	if err != nil {
		return nil, errors.E(err, "failed to open cfg directory")
	}

	defer func() {
		err = errors.L(err, f.Close()).AsError()
	}()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return nil, errors.E(err, "failed to read files in %s", cfgdir)
	}

	for _, dirEntry := range dirEntries {
		fname := dirEntry.Name()
		if fname == SkipFilename {
			logger.Debug().Msg("skip file found: skipping whole subtree")
			return NewTree(cfgdir), nil
		}
	}

	if parentTree != nil && rootcfg == nil {
		rootcfg = &parentTree.RootTree().Node
	}

	if cfgdir != parentTree.RootDir() {
		tree := NewTree(cfgdir)

		cfg, err := hcl.ParseDir(parentTree.RootDir(), cfgdir, rootcfg.Experiments()...)
		if err != nil {
			return nil, err
		}

		tree.Node = cfg
		tree.Parent = parentTree
		parentTree.Children[filepath.Base(cfgdir)] = tree

		parentTree = tree
	}

	err = processTmGenFiles(parentTree.RootTree(), &parentTree.Node, cfgdir, dirEntries)
	if err != nil {
		return nil, err
	}

	for _, dirEntry := range dirEntries {
		fname := dirEntry.Name()
		if Skip(fname) || !dirEntry.IsDir() {
			continue
		}

		dir := filepath.Join(cfgdir, fname)
		node, err := loadTree(parentTree, dir, rootcfg)
		if err != nil {
			return nil, errors.E(err, "loading from %s", dir)
		}

		node.Parent = parentTree
		parentTree.Children[fname] = node
	}
	return parentTree, nil
}

func processTmGenFiles(rootTree *Tree, cfg *hcl.Config, cfgdir string, dirEntries []fs.DirEntry) error {
	const tmgenSuffix = ".tmgen"

	tmgenEnabled := rootTree.hasExperiment("tmgen")

	// process all .tmgen files.
	for _, dirEntry := range dirEntries {
		fname := dirEntry.Name()
		if !strings.HasSuffix(fname, tmgenSuffix) || fname[0] == '.' {
			continue
		}

		absFname := filepath.Join(cfgdir, fname)

		if !tmgenEnabled {
			printer.Stderr.Warn(
				fmt.Sprintf("found %q but `tmgen` is not enabled in the `terramate.config.experiments` attribute",
					absFname,
				),
			)
			continue
		}

		content, err := os.ReadFile(absFname)
		if err != nil {
			return errors.E(err, "failed to read .tmgen file")
		}
		parser := hclparse.NewParser()
		hclFile, diags := parser.ParseHCL(content, fname)
		if diags.HasErrors() {
			return errors.E(diags, "failed to parse .tmgen file")
		}

		lines := bytes.Split(content, []byte{'\n'})
		nLines := len(lines)
		lastLineNColumns := len(lines[nLines-1])

		label := fname[:len(fname)-len(tmgenSuffix)] // removes suffix

		body, ok := hclFile.Body.(*hclsyntax.Body)
		if !ok {
			panic(errors.E(errors.ErrInternal, "unexpected parsed HCL body"))
		}

		block := &hhcl.Block{
			Type: "content",
			Body: body,
		}

		// should never fail
		inheritFalse, diags := hclsyntax.ParseExpression([]byte("false"), "<implicit block>", hhcl.InitialPos)
		if diags.HasErrors() {
			panic(errors.E(errors.ErrInternal, diags))
		}

		inheritAttr := &hclsyntax.Attribute{
			Name: "inherit",
			Expr: inheritFalse,
		}

		implicitGenBlock := hcl.GenHCLBlock{
			IsImplicitBlock: true,
			Dir:             project.PrjAbsPath(rootTree.HostDir(), cfgdir),
			Inherit:         inheritAttr,
			Range: info.NewRange(rootTree.HostDir(), hhcl.Range{
				Filename: absFname,
				Start:    hhcl.InitialPos,
				End: hhcl.Pos{
					Line:   nLines,
					Column: lastLineNColumns,
					Byte:   len(content) - 1,
				},
			}),
			Label:   label,
			Lets:    ast.NewMergedBlock("lets", []string{}),
			Asserts: nil,
			Content: block,
		}

		cfg.Generate.HCLs = append(cfg.Generate.HCLs, implicitGenBlock)
	}
	return nil
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

func (tree *Tree) hasExperiment(name string) bool {
	if tree.Parent != nil {
		return tree.Parent.hasExperiment(name)
	}
	if tree.Node.Terramate == nil || tree.Node.Terramate.Config == nil {
		return false
	}

	return slices.Contains(tree.Node.Terramate.Config.Experiments, name)
}

// HasExperiment returns true if the given experiment name is set.
func (root *Root) HasExperiment(name string) bool {
	return root.tree.hasExperiment(name)
}

// TerragruntEnabledOption returns the configured `terramate.config.change_detection.terragrunt.enabled` option.
func (root *Root) TerragruntEnabledOption() hcl.TerragruntChangeDetectionEnabledOption {
	if root.tree.Node.Terramate != nil &&
		root.tree.Node.Terramate.Config != nil &&
		root.tree.Node.Terramate.Config.ChangeDetection != nil &&
		root.tree.Node.Terramate.Config.ChangeDetection.Terragrunt != nil {
		return root.tree.Node.Terramate.Config.ChangeDetection.Terragrunt.Enabled
	}
	return hcl.TerragruntAutoOption // "auto" is the default.
}

// HasTerragruntStacks returns true if the stack loading has detected Terragrunt files.
func (root *Root) HasTerragruntStacks() bool {
	b := root.hasTerragruntStacks
	if b == nil {
		panic(errors.E(errors.ErrInternal, "root.HasTerragruntStacks should be called after stacks list is computed"))
	}
	return *b
}

// IsTerragruntChangeDetectionEnabled returns true if Terragrunt change detection integration
// must be executed.
func (root *Root) IsTerragruntChangeDetectionEnabled() bool {
	switch opt := root.TerragruntEnabledOption(); opt {
	case hcl.TerragruntOffOption:
		return false
	case hcl.TerragruntForceOption:
		return true
	case hcl.TerragruntAutoOption:
		return root.HasTerragruntStacks()
	default:
		panic(errors.E(errors.ErrInternal, "unexpected terragrunt option: %v", opt))
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

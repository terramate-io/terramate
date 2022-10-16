// Copyright 2022 Mineiros GmbH
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

package globals

import (
	"sort"
	"strings"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrEval      errors.Kind = "global eval failed"
	ErrRedefined errors.Kind = "global redefined"
)

type (
	// Expr is an unevaluated global expression.
	Expr struct {
		// Origin is the filename where this expression can be found.
		Origin project.Path

		hhcl.Expression
	}

	// Exprs is the map of unevaluated global expressions visible in a
	// directory.
	Exprs map[string]Expr

	// Value is an evaluated global.
	Value struct {
		Origin project.Path

		cty.Value
	}

	// Map is an evaluated globals map.
	Map map[string]Value

	loader struct {
		ctx           *eval.Context
		tree          *config.Tree
		dir           project.Path
		loaded        Map
		pendingBlocks []*ast.MergedBlock
		pendingAttrs  []pendingAttr
		globalsTree   *objtree
	}

	pendingAttr struct {
		path string
		attr ast.Attribute
	}
)

func newLoader(ctx *eval.Context, tree *config.Tree, cfgdir project.Path) *loader {
	_, ok := ctx.GetNamespace("globals")
	if !ok {
		ctx.SetNamespace("global", map[string]cty.Value{})
	}
	globalsTree := newobjtree()
	globalVals, _ := ctx.Globals()
	globalsTree.set(globalVals.AsValueMap())
	return &loader{
		ctx:         ctx,
		tree:        tree,
		dir:         cfgdir,
		globalsTree: globalsTree,
	}
}

// Load loads and evaluates all globals expressions defined for
// the given directory path. It will navigate the config tree from dir until it
// reaches rootdir, loading globals expressions and merging them appropriately.
// More specific globals (closer or at the dir) have precedence over less
// specific globals (closer or at the root dir).
func Load(tree *config.Tree, cfgdir project.Path, ctx *eval.Context) (EvalReport, error) {
	loader := newLoader(ctx, tree, cfgdir)
	err := loader.loadall()
	if err != nil {
		return EvalReport{}, err
	}

	// todo
	return NewEvalReport(), nil
}

func (loader *loader) loadall() error {
	tree := loader.tree
	cfgdir := loader.dir

	for {
		logger := log.With().
			Str("action", "loader.load()").
			Stringer("cfgdir", cfgdir).
			Logger()

		logger.Trace().Msg("lookup config")

		var ok bool
		cfg, ok := tree.Lookup(cfgdir)
		if !ok {
			return errors.E("configuration path %s not found", cfgdir.String())
		}

		err := loader.load(cfg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (loader *loader) load(cfg *config.Tree) error {
	globalsLabelledBlocks := cfg.Node.Globals
	var globalsBlocks []*ast.MergedBlock
	for _, globalBlock := range globalsLabelledBlocks {
		globalsBlocks = append(globalsBlocks, globalBlock)
	}

	sort.Slice(globalsBlocks, func(i, j int) bool {
		return globalsBlocks[i].Labels < globalsBlocks[j].Labels
	})

	ctx := loader.ctx
	errs := errors.L()
	globals := loader.globalsTree
	for _, globalBlock := range globalsBlocks {
		for _, attr := range globalBlock.Attributes.SortedList() {
			if _, ok := globals.has(globalBlock.Labels + "." + attr.Name); ok {
				// attr already set by higher scope
				continue
			}
			if eval.DependsOnUnknowns(attr.Expr, ctx.Raw()) {
				loader.pendingAttrs = append(loader.pendingAttrs, pendingAttr{
					path: globalBlock.Labels,
					attr: attr,
				})
				continue
			}
			val, err := ctx.Eval(attr.Expr)
			if err != nil {
				return errors.E(attr.Range, err)
			}
			loader.setGlobal(globalBlock.Labels+"."+attr.Name, val)
		}
	}

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("evaluating pending expressions")

	pendingExpression:
		for name, expr := range pendingExprs {
			logger := logger.With().
				Stringer("origin", expr.Origin).
				Str("global", name).
				Logger()

			vars := expr.Variables()

			pendingExprsErrs[name] = errors.L()

			logger.Trace().Msg("checking var access inside expression")

			for _, namespace := range vars {
				if !ctx.HasNamespace(namespace.RootName()) {
					pendingExprsErrs[name].Append(errors.E(
						ErrEval,
						namespace.SourceRange(),
						"unknown variable namespace: %s", namespace.RootName(),
					))

					continue
				}

				if namespace.RootName() != "global" {
					continue
				}

				switch attr := namespace[1].(type) {
				case hhcl.TraverseAttr:
					if _, isPending := pendingExprs[attr.Name]; isPending {
						continue pendingExpression
					}
				default:
					panic("unexpected type of traversal - this is a BUG")
				}
			}

			if err := pendingExprsErrs[name].AsError(); err != nil {
				continue
			}

			logger.Trace().Msg("evaluating expression")

			val, err := ctx.Eval(expr)
			if err != nil {
				pendingExprsErrs[name].Append(err)
				continue
			}

			globals[name] = Value{
				Origin: expr.Origin,
				Value:  val,
			}

			amountEvaluated++

			delete(pendingExprs, name)
			delete(pendingExprsErrs, name)

			logger.Trace().Msg("updating globals eval context with evaluated attribute")

			ctx.SetNamespace("global", globals.Attributes())
		}

		if amountEvaluated == 0 {
			break
		}
	}

	for name, expr := range pendingExprs {
		err := pendingExprsErrs[name].AsError()
		if err == nil {
			err = errors.E(expr.Range(), "undefined global %s", name)
		}
		report.Errors[name] = EvalError{
			Expr: expr,
			Err:  errors.E(ErrEval, err),
		}
	}

	return report

	logger.Trace().Msg("loading expressions")

	exprs := make(Exprs)

	cfg, ok := tree.Lookup(cfgdir)
	if !ok {
		return exprs, nil
	}

	globalsBlock := cfg.Node.Globals
	if len(globalsBlock) > 0 {
		logger.Trace().Msg("Range over attributes.")

		for _, attr := range globalsBlock.Attributes {
			logger.Trace().Msg("Add attribute to globals.")

			exprs[attr.Name] = Expr{
				Origin:     project.PrjAbsPath(tree.RootDir(), attr.Origin),
				Expression: attr.Expr,
			}
		}
	}

	importedGlobals, ok := cfg.Node.Imported.MergedBlocks["globals"]
	if ok {
		logger.Trace().Msg("Range over imported globals")

		importedExprs := make(Exprs)
		for _, attr := range importedGlobals.Attributes {
			logger.Trace().Msg("Add imported attribute to globals.")

			importedExprs[attr.Name] = Expr{
				Origin:     project.PrjAbsPath(tree.RootDir(), attr.Origin),
				Expression: attr.Expr,
			}
		}

		exprs.merge(importedExprs)
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return exprs, nil
	}

	logger.Trace().Msg("Loading stack globals from parent dir.")

	parentGlobals, err := LoadExprs(tree, parentcfg)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Merging globals with parent.")

	exprs.merge(parentGlobals)
	return exprs, nil
}

func (loader *loader) setGlobal(path string, val cty.Value) error {
	return loader.globalsTree.setAt(path, val)
}

type objtree struct {
	value    cty.Value // not object values
	children map[string]*objtree
}

func newobjtree() *objtree {
	return &objtree{
		children: make(map[string]*objtree),
	}
}

func (obj *objtree) set(m map[string]cty.Value) {
	for k, v := range m {
		subobj := newobjtree()
		if v.Type().IsObjectType() {
			subobj.set(v.AsValueMap())
		} else {
			subobj.value = v
		}
		obj.children[k] = subobj
	}
}

func (obj *objtree) setAt(path string, val cty.Value) error {
	pathParts := strings.Split(path, ".")
	for len(pathParts) > 1 {
		subtree, ok := obj.children[pathParts[0]]
		if !ok {
			subtree = newobjtree()
			obj.children[pathParts[0]] = subtree
		}
		if !subtree.value.IsNull() && !subtree.value.Type().IsObjectType() {
			return errors.E("cannot extend value of type %s", subtree.value.Type().FriendlyName())
		}
		obj = subtree
		pathParts = pathParts[1:]
	}

	obj.children[pathParts[0]]
	return nil
}

func (obj *object) Set(path string, value cty.Value) {

}

func (globalExprs Exprs) merge(other Exprs) {
	for k, v := range other {
		if _, ok := globalExprs[k]; !ok {
			globalExprs[k] = v
		}
	}
}

// String provides a string representation of the evaluated globals.
func (globals Map) String() string {
	return hcl.FormatAttributes(globals.Attributes())
}

// Attributes returns all the global attributes, the key in the map
// is the attribute name with its corresponding value mapped
func (globals Map) Attributes() map[string]cty.Value {
	attrcopy := map[string]cty.Value{}
	for k, v := range globals {
		attrcopy[k] = v.Value
	}
	return attrcopy
}

func removeUnset(exprs Exprs) {
	for name, expr := range exprs {
		traversal, diags := hhcl.AbsTraversalForExpr(expr.Expression)
		if diags.HasErrors() {
			continue
		}
		if len(traversal) != 1 {
			continue
		}
		if traversal.RootName() == "unset" {
			delete(exprs, name)
		}
	}
}

func parentDir(dir project.Path) (project.Path, bool) {
	parent := dir.Dir()
	return parent, parent != dir
}

func copyexprs(dst, src Exprs) {
	for k, v := range src {
		dst[k] = v
	}
}

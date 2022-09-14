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
	"path/filepath"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrGlobalEval      errors.Kind = "globals eval failed"
	ErrGlobalRedefined errors.Kind = "global redefined"
)

type (
	// Expr is an unevaluated global expression.
	Expr struct {
		// Origin is the filename where this expression can be found.
		Origin string

		hclsyntax.Expression
	}

	// Exprs is the map of unevaluated global expressions visible in a
	// directory.
	Exprs map[string]Expr

	// Value is an evaluated global.
	Value struct {
		Origin string

		cty.Value
	}

	// G is an evaluated globals map.
	G map[string]Value
)

// LoadExprs loads from the file system all globals expressions defined for
// the given directory. It will navigate the file system from dir until it
// reaches rootdir, loading globals expressions and merging them appropriately.
// More specific globals (closer or at the dir) have precedence over less
// specific globals (closer or at the root dir).
func LoadExprs(rootdir string, cfgdir string) (Exprs, error) {
	logger := log.With().
		Str("action", "globals.LoadExprs()").
		Str("root", rootdir).
		Str("cfgdir", cfgdir).
		Logger()

	logger.Debug().Msg("Parse globals blocks.")

	absdir := filepath.Join(rootdir, cfgdir)
	p, err := hcl.NewTerramateParser(rootdir, absdir)
	if err != nil {
		return nil, err
	}
	err = p.AddDir(absdir)
	if err != nil {
		return nil, errors.E("adding dir to parser", err)
	}

	err = p.Parse()
	if err != nil {
		return nil, errors.E("parsing config", err)
	}

	exprs := make(Exprs)
	globalsBlock, ok := p.Config.MergedBlocks["globals"]
	if ok {
		logger.Trace().Msg("Range over attributes.")

		for _, attr := range globalsBlock.Attributes.SortedList() {
			logger.Trace().Msg("Add attribute to globals.")

			exprs[attr.Name] = Expr{
				Origin:     project.PrjAbsPath(rootdir, attr.Origin),
				Expression: attr.Expr,
			}
		}
	}

	importedGlobals, ok := p.Imported.MergedBlocks["globals"]
	if ok {
		logger.Trace().Msg("Range over imported globals")

		importedExprs := make(Exprs)
		for _, attr := range importedGlobals.Attributes.SortedList() {
			logger.Trace().Msg("Add imported attribute to globals.")

			importedExprs[attr.Name] = Expr{
				Origin:     project.PrjAbsPath(rootdir, attr.Origin),
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

	parentGlobals, err := LoadExprs(rootdir, parentcfg)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Merging globals with parent.")

	exprs.merge(parentGlobals)
	return exprs, nil
}

// Eval will evaluate all globals.
func (globalExprs Exprs) Eval(ctx *eval.Context) (G, error) {
	logger := log.With().
		Str("action", "Exprs.Eval()").
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	globals := make(G)
	pendingExprsErrs := map[string]error{}
	pendingExprs := make(Exprs)

	copyexprs(pendingExprs, globalExprs)
	removeUnset(pendingExprs)

	if !ctx.HasNamespace("global") {
		ctx.SetNamespace("global", map[string]cty.Value{})
	}

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("evaluating pending expressions")

	pendingExpression:
		for name, expr := range pendingExprs {
			logger := logger.With().
				Str("origin", expr.Origin).
				Str("global", name).
				Logger()

			vars := hclsyntax.Variables(expr)

			logger.Trace().Msg("checking var access inside expression")

			for _, namespace := range vars {
				if !ctx.HasNamespace(namespace.RootName()) {
					return G{}, errors.E(
						ErrGlobalEval,
						namespace.SourceRange(),
						"unknown variable namespace: %s", namespace.RootName(),
					)
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

			logger.Trace().Msg("evaluating expression")

			val, err := ctx.Eval(expr)
			if err != nil {
				pendingExprsErrs[name] = err
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

	if len(pendingExprs) > 0 {
		// TODO(katcipis): model proper error list and return that
		// Caller can decide how to format/log things (like code generation report).
		pendingGlobalNames := make([]string, 0, len(pendingExprs))
		for name, expr := range pendingExprs {
			err, ok := pendingExprsErrs[name]
			if !ok {
				err = errors.E("undefined global")
			}
			pendingGlobalNames = append(pendingGlobalNames, name)
			logger.Err(err).
				Str("name", name).
				Str("origin", expr.Origin).
				Msg("evaluating global")
		}
		return G{}, errors.E(
			ErrGlobalEval, "pending unknown globals: %v", pendingGlobalNames,
		)
	}

	return globals, nil
}

func (globalExprs Exprs) merge(other Exprs) {
	for k, v := range other {
		if _, ok := globalExprs[k]; !ok {
			globalExprs[k] = v
		}
	}
}

// String provides a string representation of the evaluated globals.
func (globals G) String() string {
	return hcl.FormatAttributes(globals.Attributes())
}

// Attributes returns all the global attributes, the key in the map
// is the attribute name with its corresponding value mapped
func (globals G) Attributes() map[string]cty.Value {
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

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}

func copyexprs(dst, src Exprs) {
	for k, v := range src {
		dst[k] = v
	}
}

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

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrEval      errors.Kind = "global eval"
	ErrRedefined errors.Kind = "global redefined"
)

type (
	// Expr is an unevaluated global expression.
	Expr struct {
		// Origin is the filename where this expression can be found.
		Origin project.Path

		DotPath eval.DotPath

		hhcl.Expression
	}

	// Exprs is the map of unevaluated global expressions visible in a
	// directory.
	Exprs map[eval.DotPath]Expr
)

// Load loads all the globals from the cfgdir.
func Load(tree *config.Tree, cfgdir project.Path, ctx *eval.Context) EvalReport {
	exprs, err := LoadExprs(tree, cfgdir)
	if err != nil {
		report := NewEvalReport()
		report.BootstrapErr = err
		return report
	}

	return exprs.Eval(ctx)
}

// LoadExprs loads from the file system all globals expressions defined for
// the given directory. It will navigate the file system from dir until it
// reaches rootdir, loading globals expressions and merging them appropriately.
// More specific globals (closer or at the dir) have precedence over less
// specific globals (closer or at the root dir).
func LoadExprs(tree *config.Tree, cfgdir project.Path) (Exprs, error) {
	logger := log.With().
		Str("action", "globals.LoadExprs()").
		Str("root", tree.RootDir()).
		Stringer("cfgdir", cfgdir).
		Logger()

	exprs := make(Exprs)

	cfg, ok := tree.Lookup(cfgdir)
	if !ok {
		return exprs, nil
	}

	globalsBlocks := cfg.Node.Globals.AsList()
	for _, block := range globalsBlocks {
		logger.Trace().Msg("Range over attributes.")

		for _, attr := range block.Attributes.SortedList() {
			logger.Trace().Msg("Add attribute to globals.")

			acessor := dotpath(block.Labels, attr.Name)
			exprs[acessor] = Expr{
				Origin:     attr.Range.Path(),
				DotPath:    acessor,
				Expression: attr.Expr,
			}
		}
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

// Eval evaluates all global expressions and returns an EvalReport.
func (globalExprs Exprs) Eval(ctx *eval.Context) EvalReport {
	logger := log.With().
		Str("action", "Exprs.Eval()").
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	report := NewEvalReport()
	globals := report.Globals
	pendingExprsErrs := map[eval.DotPath]*errors.List{}
	pendingExprs := make(Exprs)

	removeUnset(globalExprs)
	copyexprs(pendingExprs, globalExprs)

	if !ctx.HasNamespace("global") {
		ctx.SetNamespace("global", map[string]cty.Value{})
	}

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("evaluating pending expressions")

		sortedKeys := []eval.DotPath{}
		for accessor := range pendingExprs {
			sortedKeys = append(sortedKeys, accessor)
		}

		sort.Slice(sortedKeys, func(i, j int) bool {
			return sortedKeys[i] < sortedKeys[j]
		})

	pendingExpression:
		for _, accessor := range sortedKeys {
			expr := pendingExprs[accessor]

			logger := logger.With().
				Stringer("origin", expr.Origin).
				Str("global", string(accessor)).
				Logger()

			vars := expr.Variables()

			pendingExprsErrs[accessor] = errors.L()

			logger.Trace().Msg("checking var access inside expression")

			for _, namespace := range vars {
				if !ctx.HasNamespace(namespace.RootName()) {
					pendingExprsErrs[accessor].Append(errors.E(
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
					if _, isPending := pendingExprs[eval.DotPath(attr.Name)]; isPending {
						continue pendingExpression
					}
				default:
					panic("unexpected type of traversal - this is a BUG")
				}
			}

			if err := pendingExprsErrs[accessor].AsError(); err != nil {
				continue
			}

			// This catches a schema error that cannot be detected at the parser.
			// When a nested object is defined either by literal or funcalls,
			// it can't be detected at the parser.
			if _, ok := globals.GetKeyPath(accessor); ok {
				pendingExprsErrs[accessor].Append(
					errors.E(hcl.ErrTerramateSchema, expr.Range(),
						"global.%s attribute redefined",
						accessor))
				continue
			}

			logger.Trace().Msg("evaluating expression")

			val, err := ctx.Eval(expr)
			if err != nil {
				pendingExprsErrs[accessor].Append(errors.E(ErrEval, err, "global.%s", accessor))
				continue
			}

			err = globals.SetAt(accessor, eval.NewValue(val, expr.Origin))
			if err != nil {
				pendingExprsErrs[accessor].Append(err)
				continue
			}

			amountEvaluated++

			delete(pendingExprs, accessor)
			delete(pendingExprsErrs, accessor)

			logger.Trace().Msg("updating globals eval context with evaluated attribute")

			ctx.SetNamespace("global", globals.AsValueMap())
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
}

func (globalExprs Exprs) merge(other Exprs) {
	for k, v := range other {
		if _, ok := globalExprs[k]; !ok {
			globalExprs[k] = v
		}
	}
}

// TODO(i4k): review this
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

func dotpath(basepath string, name string) eval.DotPath {
	if basepath != "" {
		return eval.DotPath(basepath + "." + name)
	}
	return eval.DotPath(name)
}

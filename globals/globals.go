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
	"github.com/hashicorp/hcl/v2/hclsyntax"
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

		// LabelPath denotes the target accessor path which the expression must
		// be assigned into.
		LabelPath eval.ObjectPath

		hhcl.Expression
	}

	// GlobalPathKey represents a global object accessor to be used as map key.
	// The reason is that slices cannot be used as map key because the equality
	// operator is not defined, then this type implements a fixed size struct.
	GlobalPathKey struct {
		path     [project.MaxGlobalLabels]string
		isattr   bool
		numPaths int
	}
)

// Path returns the global accessor path (labels + attribute name).
func (a GlobalPathKey) Path() []string { return a.path[:a.numPaths] }

func (a GlobalPathKey) rootname() string {
	if a.numPaths == 0 {
		return ""
	}
	return a.path[0]
}

// Load loads all the globals from the cfgdir.
func Load(tree *config.Root, cfgdir project.Path, ctx *eval.Context) EvalReport {
	logger := log.With().
		Str("action", "globals.Load()").
		Str("root", tree.Dir()).
		Stringer("cfgdir", cfgdir).
		Logger()

	logger.Trace().Msg("loading expressions")

	exprs, err := loadExprs(tree, cfgdir)
	if err != nil {
		report := NewEvalReport()
		report.BootstrapErr = err
		return report
	}

	return exprs.eval(ctx)
}

// exprSet represents a set of globals loaded from a dir.
// The origin is the path of the dir from where all expressions were loaded.
type exprSet struct {
	origin      project.Path
	expressions map[GlobalPathKey]Expr
}

// loadedExprs contains all loaded global expressions from multiple configuration
// directories. Each configuration dir path is mapped to its loaded global expressions.
type loadedExprs map[project.Path]exprSet

func newExprSet(origin project.Path) exprSet {
	return exprSet{
		origin:      origin,
		expressions: map[GlobalPathKey]Expr{},
	}
}

// loadExprs loads from the file system all globals expressions defined for
// the given directory. It will navigate the file system from dir until it
// reaches rootdir, loading globals expressions and merging them appropriately.
// More specific globals (closer or at the dir) have precedence over less
// specific globals (closer or at the root dir).
func loadExprs(tree *config.Root, cfgdir project.Path) (loadedExprs, error) {
	logger := log.With().
		Str("action", "globals.loadExprs()").
		Str("root", tree.Dir()).
		Stringer("cfgdir", cfgdir).
		Logger()

	exprs := newExprSet(cfgdir)

	cfg, ok := tree.Lookup(cfgdir)
	if !ok {
		return loadedExprs{}, nil
	}

	globalsBlocks := cfg.Node.Globals.AsList()
	for _, block := range globalsBlocks {
		if len(block.Labels) > 0 && !hclsyntax.ValidIdentifier(block.Labels[0]) {
			return nil, errors.E(
				hcl.ErrTerramateSchema,
				"first global label must be a valid identifier but got %s",
				block.Labels[0],
			)
		}

		attrs := block.Attributes.SortedList()
		if len(block.Labels) > 0 && len(attrs) == 0 {
			expr, _ := eval.ParseExpressionBytes([]byte(`{}`))
			key := newGlobalPath(block.Labels, "")
			exprs.expressions[key] = Expr{
				Origin:     block.RawOrigins[0].Range.Path(),
				LabelPath:  key.Path(),
				Expression: expr,
			}
		}

		logger.Trace().Msg("Range over attributes.")

		for _, attr := range attrs {
			logger.Trace().Msg("Add attribute to globals.")

			key := newGlobalPath(block.Labels, attr.Name)
			exprs.expressions[key] = Expr{
				Origin:     attr.Range.Path(),
				LabelPath:  key.Path(),
				Expression: attr.Expr,
			}
		}
	}

	globals := loadedExprs{
		cfgdir: exprs,
	}

	parentcfg, ok := parentDir(cfgdir)
	if !ok {
		return globals, nil
	}

	logger.Trace().Msg("Loading stack globals from parent dir.")

	parentGlobals, err := loadExprs(tree, parentcfg)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Merging globals with parent.")

	globals.merge(parentGlobals)
	return globals, nil
}

// Returns a sorted loaded exprs, sorting it by config dir path.
// The loaded expressions are sorted by the config dir path
// from smaller (root) to more specific (stack). Eg:
// - /
// - /dir
// - /dir/stack
func (le loadedExprs) sort() []exprSet {
	cfgdirs := []project.Path{}
	for cfgdir := range le {
		cfgdirs = append(cfgdirs, cfgdir)
	}

	sort.SliceStable(cfgdirs, func(i, j int) bool {
		return len(cfgdirs[i]) < len(cfgdirs[j])
	})

	res := []exprSet{}
	for _, cfgdir := range cfgdirs {
		res = append(res, le[cfgdir])
	}
	return res
}

// Returns the expressions access path sorted from the smallest to
// the biggest path.
// - global.a
// - global.a.b
// - global.a.b.c
func (es exprSet) sort() []GlobalPathKey {
	res := []GlobalPathKey{}
	for globalPath := range es.expressions {
		res = append(res, globalPath)
	}

	sort.SliceStable(res, func(i, j int) bool {
		return len(res[i].Path()) < len(res[j].Path())
	})

	return res
}

// eval evaluates all global expressions and returns an EvalReport.
func (le loadedExprs) eval(ctx *eval.Context) EvalReport {
	logger := log.With().
		Str("action", "Exprs.Eval()").
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	report := NewEvalReport()
	globals := report.Globals
	pendingExprsErrs := map[GlobalPathKey]*errors.List{}

	sortedLoadedExprs := le.sort()
	pendingExprs := map[GlobalPathKey]Expr{}

	// Here we will override values, but since
	// we ordered by config dir the more specific global expressions
	// will override the parent ones.
	for _, exprs := range sortedLoadedExprs {
		for k, v := range exprs.expressions {
			pendingExprs[k] = v
		}
	}

	// Here we will sort each set of globals from each dir independently
	// So the final iteration order is parent first then child, and
	// for each given config dir it is ordered by the length of the global path.
	// So we guarantee that independent of the expression accessors length we always
	// process parent expressions first, then the child ones, until reaching the stack.
	type globalAccessors struct {
		origin    project.Path
		accessors []GlobalPathKey
	}
	sortedGlobalAccessors := []globalAccessors{}
	for _, exprset := range sortedLoadedExprs {
		// for now we are allowing repeated access paths for different
		// directories, should not affect results since pendingExprs already
		// has the correct expression anyway.
		sortedGlobalAccessors = append(sortedGlobalAccessors, globalAccessors{
			origin:    exprset.origin,
			accessors: exprset.sort(),
		})
	}

	if !ctx.HasNamespace("global") {
		ctx.SetNamespace("global", map[string]cty.Value{})
	}

	for len(pendingExprs) > 0 {
		amountEvaluated := 0

		logger.Trace().Msg("evaluating pending expressions")

		for _, sortedGlobals := range sortedGlobalAccessors {

		pendingExpression:
			for _, accessor := range sortedGlobals.accessors {
				expr, ok := pendingExprs[accessor]
				if !ok {
					// Ignoring already evaluated expression
					continue
				}

				logger := logger.With().
					Stringer("origin", sortedGlobals.origin).
					Strs("global", accessor.Path()).
					Logger()

				logger.Trace().Msg("checking var access inside expression")

				traversal, diags := hhcl.AbsTraversalForExpr(expr.Expression)
				if !diags.HasErrors() && len(traversal) == 1 && traversal.RootName() == "unset" {
					if _, ok := globals.GetKeyPath(accessor.Path()); ok {
						err := globals.DeleteAt(accessor.Path())
						if err != nil {
							panic(errors.E(errors.ErrInternal, err))
						}
					}

					amountEvaluated++
					delete(pendingExprs, accessor)
					delete(pendingExprsErrs, accessor)
				}

				pendingExprsErrs[accessor] = errors.L()
				for _, namespace := range expr.Variables() {
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
						if _, isPending := pendingExprs[newGlobalPath([]string{}, attr.Name)]; isPending {
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
				oldValue, hasOldValue := globals.GetKeyPath(accessor.Path())
				if hasOldValue &&
					accessor.isattr &&
					oldValue.Origin().Dir().String() == expr.Origin.Dir().String() {
					pendingExprsErrs[accessor].Append(
						errors.E(hcl.ErrTerramateSchema, expr.Range(),
							"global.%s attribute redefined: previously defined at %s",
							accessor.rootname(), oldValue.Origin().String()))

					continue
				}

				logger.Trace().Msg("evaluating expression")

				val, err := ctx.Eval(expr)
				if err != nil {
					pendingExprsErrs[accessor].Append(errors.E(
						ErrEval, err, "global.%s (%t)", accessor.rootname(), accessor.isattr))
					continue
				}

				if !hasOldValue || !oldValue.IsObject() || accessor.isattr {
					// all the `attr = expr` inside global blocks become an entry
					// in the globalExprs map but we have the special case that
					// an empty globals block with labels must implicitly create
					// the label defined object...
					// then as it does not define any expression, an implicit
					// expression for an empty object block is added to the map.
					// This special entry sets the key accessor.isattr = false
					// which means this expression doesn't come from an attribute.

					// this `if` happens for the general case, which we must set the
					// actual value and then ignores the case where it has a fake
					// expression when extending an existing object.
					logger.Trace().Msg("setting global")

					err = globals.SetAt(accessor.Path(), eval.NewValue(val, expr.Origin))
					if err != nil {
						pendingExprsErrs[accessor].Append(errors.E(err, "setting global"))
						continue
					}
				} else {
					logger.Trace().Msg("ignoring implicitly created empty global")
				}

				amountEvaluated++

				delete(pendingExprs, accessor)
				delete(pendingExprsErrs, accessor)

				logger.Trace().Msg("updating globals eval context with evaluated attribute")

				ctx.SetNamespace("global", globals.AsValueMap())
			}
		}

		if amountEvaluated == 0 {
			break
		}
	}

	for accessor, expr := range pendingExprs {
		err := pendingExprsErrs[accessor].AsError()
		if err == nil {
			err = errors.E(expr.Range(), "undefined global.%s", accessor.rootname())
		}
		report.Errors[accessor] = EvalError{
			Expr: expr,
			Err:  errors.E(ErrEval, err),
		}
	}

	return report
}

func (le loadedExprs) merge(other loadedExprs) {
	for k, v := range other {
		if _, ok := le[k]; !ok {
			le[k] = v
		} else {
			panic(errors.E(errors.ErrInternal, "cant merge duplicated configuration %q", k))
		}
	}
}

func parentDir(dir project.Path) (project.Path, bool) {
	parent := dir.Dir()
	return parent, parent != dir
}

func newGlobalPath(basepath []string, name string) GlobalPathKey {
	accessor := GlobalPathKey{}
	accessor.numPaths = len(basepath)
	copy(accessor.path[:], basepath)
	if name != "" {
		accessor.path[len(basepath)] = name
		accessor.numPaths++
		accessor.isattr = true
	}
	return accessor
}

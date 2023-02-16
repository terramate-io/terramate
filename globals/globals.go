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
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/mapexpr"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
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
		Origin info.Range

		// ConfigDir is the directory which loaded this expression.
		ConfigDir project.Path

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

func (a GlobalPathKey) name() string {
	return strings.Join(a.path[:a.numPaths], ".")
}

// ForDir loads all the globals from the cfgdir.
// It will navigate the configuration tree up from the dir until it reaches root,
// loading globals and merging them appropriately.
//
// More specific globals (closer or at the current dir) have precedence over
// less specific globals (closer or at the root dir).
func ForDir(root *config.Root, cfgdir project.Path, ctx *eval.Context) EvalReport {
	logger := log.With().
		Str("action", "globals.Load()").
		Str("root", root.HostDir()).
		Stringer("cfgdir", cfgdir).
		Logger()

	tree, ok := root.Lookup(cfgdir)
	if !ok {
		return NewEvalReport()
	}

	logger.Trace().Msg("loading expressions")

	exprs, err := LoadExprs(tree)
	if err != nil {
		report := NewEvalReport()
		report.BootstrapErr = err
		return report
	}

	return exprs.Eval(ctx)
}

// ExprSet represents a set of globals loaded from a dir.
// The origin is the path of the dir from where all expressions were loaded.
type ExprSet struct {
	origin      project.Path
	expressions map[GlobalPathKey]Expr
}

// HierarchicalExprs contains all loaded global expressions from multiple
// configuration directories (the key). Each configuration dir path is mapped to
// its global expressions.
type HierarchicalExprs map[project.Path]*ExprSet

func newExprSet(origin project.Path) *ExprSet {
	return &ExprSet{
		origin:      origin,
		expressions: map[GlobalPathKey]Expr{},
	}
}

// LoadExprs loads from the file system all globals expressions defined for
// the given directory. It will navigate the file system from dir until it
// reaches rootdir, loading globals expressions and merging them appropriately.
// More specific globals (closer or at the dir) have precedence over less
// specific globals (closer or at the root dir).
func LoadExprs(tree *config.Tree) (HierarchicalExprs, error) {
	logger := log.With().
		Str("action", "globals.LoadExprs()").
		Stringer("dir", tree.Dir()).
		Logger()

	exprs := newExprSet(tree.Dir())

	globalsBlocks := tree.Node.Globals.AsList()
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
			expr := &hclsyntax.ObjectConsExpr{
				SrcRange: block.RawOrigins[0].Range.ToHCLRange(),
			}
			key := NewGlobalExtendPath(block.Labels)
			exprs.expressions[key] = Expr{
				Origin:     block.RawOrigins[0].Range,
				ConfigDir:  tree.Dir(),
				LabelPath:  key.Path(),
				Expression: expr,
			}
		}

		for _, varsBlock := range block.Blocks {
			varName := varsBlock.Labels[0]
			if _, ok := block.Attributes[varName]; ok {
				return HierarchicalExprs{}, errors.E(
					ErrRedefined,
					"map label %s conflicts with global.%s attribute", varName, varName)
			}

			logger.Trace().Msgf("Add map.%s to globals", varName)

			key := NewGlobalAttrPath(block.Labels, varName)
			expr, err := mapexpr.NewMapExpr(varsBlock)
			if err != nil {
				return HierarchicalExprs{}, errors.E(err, "failed to interpret map block")
			}
			exprs.expressions[key] = Expr{
				Origin:     varsBlock.RawOrigins[0].Range,
				LabelPath:  key.Path(),
				Expression: expr,
			}
		}

		logger.Trace().Msg("Range over attributes.")

		for _, attr := range attrs {
			logger.Trace().Msg("Add attribute to globals.")

			key := NewGlobalAttrPath(block.Labels, attr.Name)
			exprs.expressions[key] = Expr{
				Origin:     attr.Range,
				ConfigDir:  tree.Dir(),
				LabelPath:  key.Path(),
				Expression: attr.Expr,
			}
		}
	}

	globals := HierarchicalExprs{
		tree.Dir(): exprs,
	}

	parent := tree.NonEmptyGlobalsParent()
	if parent == nil {
		return globals, nil
	}

	logger.Trace().Msg("Loading stack globals from parent dir.")

	parentGlobals, err := LoadExprs(parent)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Merging globals with parent.")

	globals.merge(parentGlobals)
	return globals, nil
}

// SetOverride sets a custom global at the specified directory, using the given
// global path and expr. The origin is only used for debugging purposes.
func (dirExprs HierarchicalExprs) SetOverride(
	dir project.Path,
	path GlobalPathKey,
	expr hhcl.Expression,
	origin info.Range,
) {
	exprSet, ok := dirExprs[dir]
	if !ok {
		exprSet = newExprSet(origin.Path())
		dirExprs[dir] = exprSet
	}
	exprSet.expressions[path] = Expr{
		Origin:     origin,
		ConfigDir:  dir,
		LabelPath:  path.Path(),
		Expression: expr,
	}
}

// Returns a sorted loaded exprs, sorting it by config dir path.
// The loaded expressions are sorted by the config dir path
// from smaller (root) to more specific (stack). Eg:
// - /
// - /dir
// - /dir/stack
func (dirExprs HierarchicalExprs) sort() []*ExprSet {
	cfgdirs := []project.Path{}
	for cfgdir := range dirExprs {
		cfgdirs = append(cfgdirs, cfgdir)
	}

	sort.SliceStable(cfgdirs, func(i, j int) bool {
		return len(cfgdirs[i].String()) < len(cfgdirs[j].String())
	})

	res := []*ExprSet{}
	for _, cfgdir := range cfgdirs {
		res = append(res, dirExprs[cfgdir])
	}
	return res
}

// Returns the expressions access path sorted from the smallest to
// the biggest path.
// - global.a
// - global.a.b
// - global.a.b.c
func (dirExprs ExprSet) sort() []GlobalPathKey {
	res := []GlobalPathKey{}
	for globalPath := range dirExprs.expressions {
		res = append(res, globalPath)
	}

	sort.SliceStable(res, func(i, j int) bool {
		return len(res[i].Path()) < len(res[j].Path())
	})

	return res
}

// Eval evaluates all global expressions and returns an EvalReport.
func (dirExprs HierarchicalExprs) Eval(ctx *eval.Context) EvalReport {
	logger := log.With().
		Str("action", "HierarchicalExprs.Eval()").
		Logger()

	logger.Trace().Msg("Create new evaluation context.")

	report := NewEvalReport()
	globals := report.Globals
	pendingExprsErrs := map[GlobalPathKey]*errors.List{}

	sortedLoadedExprs := dirExprs.sort()
	pendingExprs := map[GlobalPathKey]Expr{}

	// Here we will override values, but since
	// we ordered by config dir the more specific global expressions
	// will override the parent ones.
	for _, xp := range sortedLoadedExprs {
		for k, v := range xp.expressions {
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

					if namespace.RootName() != "global" || len(namespace) == 1 {
						continue
					}

					var varPaths []string

					for _, ns := range namespace[1:] {
						switch attr := ns.(type) {
						case hhcl.TraverseAttr:
							varPaths = append(varPaths, attr.Name)
						case hhcl.TraverseIndex, hhcl.TraverseSplat:
							break

						default:
							panic(errors.E(
								errors.ErrInternal,
								"unexpected type of traversal - this is a BUG: %T",
								attr,
							))
						}
					}

					for len := len(varPaths); len >= 1; len-- {
						base := varPaths[:len-1]
						attr := varPaths[len-1]

						if _, isPending := pendingExprs[newGlobalPath(base, attr)]; isPending {
							continue pendingExpression
						}
					}
				}

				// also checks if any part of the accessor is pending.
				// Example:
				//   globals a {
				//       val = tm_try(global.pending, 1)
				//   }
				//
				//   globals a b {
				//       c = 1
				//   }
				//
				// The first global block would evaluate before but as it has
				// pending variables, then we need to postpone the second block
				// as well.
				if len(accessor.Path()) > 1 {
					for size := accessor.numPaths; size >= 1; size-- {
						base := accessor.path[0 : size-1]
						attr := accessor.path[size-1]
						v, isPending := pendingExprs[newGlobalPath(base, attr)]

						if isPending &&
							// is not this global path
							!isSameObjectPath(v.LabelPath, accessor.Path()) &&
							// dependent comes from same or higher level
							strings.HasPrefix(sortedGlobals.origin.String(), v.ConfigDir.String()) {
							continue pendingExpression
						}

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
					oldValue.Info().DefinedAt.Dir().String() == expr.Origin.Path().Dir().String() {
					pendingExprsErrs[accessor].Append(
						errors.E(hcl.ErrTerramateSchema, expr.Range(),
							"global.%s attribute redefined: previously defined at %s",
							accessor.name(), oldValue.Info().DefinedAt.String()))

					continue
				}

				// This is to avoid setting a label defined extension on the child
				// and later overwriting that with an object definition on the parent

				logger.Trace().Msg("evaluating expression")

				val, err := ctx.Eval(expr)
				if err != nil {
					pendingExprsErrs[accessor].Append(errors.E(
						ErrEval, err, "global.%s (%t)", accessor.rootname(), accessor.isattr))
					continue
				}

				if hasOldValue && oldValue.IsObject() && !accessor.isattr {
					// all the `attr = expr` inside global blocks become an entry
					// in the globalExprs map but we have the special case that
					// an empty globals block with labels must implicitly create
					// the label defined object...
					// then as it does not define any expression, an implicit
					// expression for an empty object block is added to the map.
					// This special entry sets the key accessor.isattr = false
					// which means this expression doesn't come from an attribute.

					// this `if` happens for the general case, which we must not
					// set the fake expression when extending an existing object.

					logger.Trace().Msg("ignoring implicitly created empty global")
				} else {
					logger.Trace().Msg("setting global")

					err := setGlobal(globals, accessor, eval.NewValue(val,
						eval.Info{
							DefinedAt: expr.Origin.Path(),
							Dir:       sortedGlobals.origin,
						},
					))

					if err != nil {
						pendingExprsErrs[accessor].Append(errors.E(err, "setting global"))
						continue
					}
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

func (dirExprs HierarchicalExprs) merge(other HierarchicalExprs) {
	for k, v := range other {
		if _, ok := dirExprs[k]; !ok {
			dirExprs[k] = v
		} else {
			panic(errors.E(
				errors.ErrInternal,
				"cant merge duplicated configuration %q",
				k))
		}
	}
}

func isSameObjectPath(a, b eval.ObjectPath) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

// setGlobal sets the global accordingly to the hierarchical rules.
func setGlobal(globals *eval.Object, accessor GlobalPathKey, newVal eval.Value) error {
	oldVal, hasOldVal := globals.GetKeyPath(accessor.Path())
	if !hasOldVal {
		return globals.SetAt(accessor.Path(), newVal)
	}

	newConfigDir := newVal.Info().Dir
	oldConfigDir := oldVal.Info().Dir

	newDefinedDir := newVal.Info().DefinedAt.Dir()
	oldDefinedDir := oldVal.Info().DefinedAt.Dir()

	if !newConfigDir.HasPrefix(oldConfigDir.String()) {
		panic(errors.E(errors.ErrInternal,
			"unexpected globals behavior: new value from dir %s and defined at %s: "+
				"old value from dir %s and defined at %s",
			newConfigDir, newDefinedDir,
			oldConfigDir, oldDefinedDir))
	}

	// newval comes from lower layer.

	return globals.SetAt(accessor.Path(), newVal)

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

// NewGlobalAttrPath creates a new global path key for an attribute.
func NewGlobalAttrPath(basepath []string, name string) GlobalPathKey {
	return newGlobalPath(basepath, name)
}

// NewGlobalExtendPath creates a new global path key for extension purposes only.
func NewGlobalExtendPath(path []string) GlobalPathKey {
	return newGlobalPath(path, "")
}

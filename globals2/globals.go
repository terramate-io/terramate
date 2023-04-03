package globals2

import (
	"fmt"
	"strings"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/mapexpr"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrEval      errors.Kind = "global eval"
	ErrRedefined errors.Kind = "global redefined"
)

type (
	G struct {
		ctx     *eval.Context
		tree    *config.Tree
		scopes  map[project.Path]Scope
		globals *eval.Object
	}

	Scope map[Ref]hhcl.Expression

	// Ref is a variable reference.
	Ref struct {
		path     [project.MaxGlobalLabels]string
		numPaths int
	}
)

const debug = true

func New(ctx *eval.Context, tree *config.Tree) *G {
	ctx.SetNamespace("global", map[string]cty.Value{})
	return &G{
		ctx:    ctx,
		tree:   tree,
		scopes: make(map[project.Path]Scope),
		globals: eval.NewObject(eval.Info{
			Dir:       tree.Dir(),
			DefinedAt: tree.Dir(),
		}),
	}
}

func (g *G) EvalAll() (*eval.Object, error) {
	return g.evalAll(g.tree)
}

func (g *G) evalAll(tree *config.Tree) (*eval.Object, error) {
	var err error
	scope, ok := g.scopes[tree.Dir()]
	if !ok {
		scope, err = g.loadScope(tree.Node.Globals)
		if err != nil {
			return nil, err
		}
		g.scopes[tree.Dir()] = scope
	}

	for tRef, expr := range scope {
		debugf("%s: evaluating %s = %s\n", tree.Dir(), tRef, ast.TokensForExpression(expr).Bytes())
		oldv, hasOld := g.globals.GetKeyPath(tRef.InternalPath())
		if hasOld && !oldv.IsObject() {
			continue
		}

		val, err := g.Eval(expr)
		if err != nil {
			return nil, err
		}

		vval := eval.NewValue(val, g.globals.Info())

		if hasOld {
			if !vval.IsObject() {
				panic("cannot merge")
			}

			merge(oldv.(*eval.Object), vval.(*eval.Object))

			vval = oldv
		}

		err = g.globals.SetAt(tRef.InternalPath(), vval)
		if err != nil {
			return nil, err
		}
		g.ctx.SetNamespace("global", g.globals.AsValueMap())
		debugf("globals: %s\n", g.globals.String())
	}

	if tree.Parent != nil {
		return g.evalAll(tree.Parent)
	}
	return g.globals, nil
}

func merge(dst, src *eval.Object) {
	for k, v := range src.Keys {
		if vv, ok := dst.Keys[k]; ok {
			if v.IsObject() && vv.IsObject() {
				merge(vv.(*eval.Object), v.(*eval.Object))
			}
		} else {
			dst.Keys[k] = v
		}
	}
}

func (g *G) Eval(expr hhcl.Expression, ancestors ...Ref) (cty.Value, error) {
	dependencies := refs(expr.Variables())
	for _, dep := range dependencies {
		if dep.Path()[0] != "global" {
			continue
		}
		for _, anc := range ancestors {
			if dep == anc {
				return cty.NilVal, errors.E("cycle")
			}
		}
		_, found := g.globals.GetKeyPath(dep.InternalPath())
		if found {
			continue
		}
		expr, found, err := g.lookup(g.tree, dep)
		if err != nil {
			return cty.NilVal, errors.E(ErrEval, err, "looking up %v", dep)
		}
		if !found {
			return cty.NilVal, errors.E(ErrEval, "expression depends on %v: not found", dep)
		}

		newAncestors := make([]Ref, len(ancestors)+1)
		copy(newAncestors, ancestors)
		newAncestors[len(ancestors)] = dep
		val, err := g.Eval(expr, newAncestors...)
		if err != nil {
			return cty.NilVal, err
		}
		// TODO(i4k): origin is not needed anymore?
		err = g.globals.SetAt(dep.InternalPath(), eval.NewValue(val, g.globals.Info()))
		if err != nil {
			return cty.NilVal, errors.E(ErrEval, err)
		}
		g.ctx.SetNamespace("global", g.globals.AsValueMap())
	}
	return g.ctx.Eval(expr)
}

// lookup
// a.b.c
//
// /      => a.b.c = 1
// /test/ => a.z   = 1
func (g *G) lookup(tree *config.Tree, ref Ref) (hhcl.Expression, bool, error) {
	debugf("lookup %s at %s\n", ref, tree.Dir())
	scope, ok := g.scopes[tree.Dir()]
	if !ok {
		var err error
		scope, err = g.loadScope(tree.Node.Globals)
		if err != nil {
			return nil, false, err
		}
		g.scopes[tree.Dir()] = scope
	}

	debugf("get %s at %+v\n", ref, scope)
	expr, ok := scope[ref]
	if !ok {
		parent := tree.Parent
		if parent != nil {
			return g.lookup(parent, ref)
		}

		// bug.. already at root here
		query := ref
		query.numPaths--
		for query.numPaths > 1 {
			expr, ok, err := g.lookup(tree, query)
			if err != nil {
				return nil, false, err
			}
			if ok {
				debugf("found %s at %s\n", query, tree.Dir())
				return expr, true, nil
			}
			query.numPaths--
		}

		return nil, false, nil
	}
	debugf("found %s at %s\n", ref, tree.Dir())
	return expr, true, nil
}

func (g *G) loadScope(globalsBlocks ast.MergedLabelBlocks) (Scope, error) {
	scope := Scope{}
	for _, block := range globalsBlocks.AsList() {
		if len(block.Labels) > 0 && !hclsyntax.ValidIdentifier(block.Labels[0]) {
			return Scope{}, errors.E(
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
			key := newRef("global", block.Labels)
			scope[key] = expr
			debugf("   set %s = %s\n", key, ast.TokensForExpression(expr).Bytes())
		}

		for _, mapBlock := range block.Blocks {
			varName := mapBlock.Labels[0]
			if _, ok := block.Attributes[varName]; ok {
				return Scope{}, errors.E(
					ErrRedefined,
					"map label %s conflicts with global.%s attribute", varName, varName)
			}

			key := newRef("global", block.Labels, []string{varName})
			expr, err := mapexpr.NewMapExpr(mapBlock)
			if err != nil {
				return Scope{}, errors.E(err, "failed to interpret map block")
			}
			scope[key] = expr
			debugf("   set %s = %s\n", key, ast.TokensForExpression(expr).Bytes())
		}

		for _, attr := range attrs {
			key := newRef("global", block.Labels, []string{attr.Name})
			scope[key] = attr.Expr
			debugf("   set %s = %s\n", key, ast.TokensForExpression(attr.Expr).Bytes())
		}
	}

	return scope, nil
}

func refs(traversals []hhcl.Traversal) []Ref {
	ret := []Ref{}
	for _, trav := range traversals {
		path := Ref{}
		if len(trav) > project.MaxGlobalLabels {
			panic(errors.E(errors.ErrInternal, "too many paths"))
		}
	outer:
		for _, tt := range trav {
			var component string
			switch t := tt.(type) {
			case hhcl.TraverseRoot:
				component = t.Name
			case hhcl.TraverseAttr:
				component = t.Name
			case hhcl.TraverseSplat:
				break outer
			case hhcl.TraverseIndex:
				if t.Key.Type().Equals(cty.String) {
					component = t.Key.AsString()
				} else {
					break outer
				}
			default:
				panic(errors.E(errors.ErrInternal, "unexpected traversal"))
			}
			path.path[path.numPaths] = component
			path.numPaths++
		}
		ret = append(ret, path)
	}
	return ret
}

func newRef(ns string, paths ...[]string) Ref {
	accessor := Ref{}
	accessor.numPaths++ // ns
	accessor.path[0] = ns
	for _, path := range paths {
		copy(accessor.path[accessor.numPaths:], path)
		accessor.numPaths += len(path)
	}
	return accessor
}

func (r Ref) String() string { return strings.Join(r.Path(), ".") }

func (r Ref) Path() []string { return r.path[:r.numPaths] }

func (r Ref) InternalPath() []string { return r.path[1:r.numPaths] }

func debugf(format string, args ...interface{}) {
	if debug {
		fmt.Printf(format, args...)
	}
}

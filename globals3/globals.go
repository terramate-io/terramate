package globals3

import (
	"fmt"
	"sort"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrEval errors.Kind = "global eval"
)

type (
	G struct {
		ctx     *eval.Context
		tree    *config.Tree
		globals globals

		Scopes map[project.Path]Stmts
	}

	globals struct {
		byref map[RefStr]cty.Value
		bykey *orderedmap.OrderedMap[string, any]
	}

	// Stmt represents a `var-decl` stmt.
	Stmt struct {
		LHS   Ref
		RHS   hhcl.Expression
		Scope project.Path

		Special bool

		// Origin is the *origin ref*. If it's nil, then it's the same as LHS.
		Origin Ref
	}

	Stmts []Stmt
)

func New(ctx *eval.Context, tree *config.Tree) *G {
	ctx.SetNamespace("global", map[string]cty.Value{})
	return &G{
		ctx:    ctx,
		tree:   tree,
		Scopes: make(map[project.Path]Stmts),
		globals: globals{
			byref: make(map[RefStr]cty.Value),
			bykey: orderedmap.New[string, any](),
		},
	}
}

func (g *G) Context() *eval.Context { return g.ctx }

func (g *G) Eval(expr hhcl.Expression) (cty.Value, error) {
	return g.eval(expr, map[RefStr]Ref{})
}

func (g *G) eval(expr hhcl.Expression, visited map[RefStr]Ref) (cty.Value, error) {
	refs := refsOf(expr)
	for _, dep := range refs {
		if _, ok := visited[dep.AsKey()]; ok {
			return cty.NilVal, errors.E(ErrEval, "cycle detected") // TODO(i4k): improve error msg

		}
		visited[dep.AsKey()] = dep
		if _, ok := g.globals.byref[dep.AsKey()]; ok {
			// dep already evaluated.
			continue
		}

		stmts, err := g.lookupStmts(dep)
		if err != nil {
			return cty.NilVal, err
		}
		if len(stmts) == 0 {
			return cty.NilVal, errors.E(
				ErrEval,
				"evaluating %s: no global declaration found for %s",
				ast.TokensForExpression(expr).Bytes(), dep)
		}
		fmt.Printf("got %d stmts\n", len(stmts))
		for _, stmt := range stmts {
			if _, ok := g.globals.byref[stmt.LHS.AsKey()]; ok {
				// stmt already evaluated.
				// This can happen when the current scope is overriding the parent
				// object but still the target expr is looking for the entire object
				// so we still have to ascent into parent scope and then the "already
				// overriden" refs show up here.
				continue
			}
			fmt.Printf("evaluating %s\n", stmt)
			if stmt.Special {
				err := g.set(stmt.LHS, cty.ObjectVal(map[string]cty.Value{}))
				if err != nil {
					return cty.NilVal, errors.E(ErrEval, err)
				}
			} else {
				val, err := g.eval(stmt.RHS, visited)
				if err != nil {
					return cty.NilVal, errors.E(err, ErrEval, "evaluating %s from %s scope", stmt.LHS, stmt.Scope)
				}

				g.set(stmt.LHS, val)
			}
		}
	}

	g.ctx.SetNamespace("global", tocty(g.globals.bykey).AsValueMap())

	val, err := g.ctx.Eval(expr)
	if err != nil {
		return cty.NilVal, errors.E(err, ErrEval, "failed to evaluate: %s", ast.TokensForExpression(expr).Bytes())
	}
	return val, nil
}

func (g *G) loadStmts() (Stmts, error) {
	return g.loadStmtsAt(g.tree)
}

func (g *G) loadStmtsAt(tree *config.Tree) (Stmts, error) {
	stmts, ok := g.Scopes[tree.Dir()]
	if ok {
		return stmts, nil
	}
	for _, block := range tree.Node.Globals.AsList() {
		if len(block.Labels) > 0 && !hclsyntax.ValidIdentifier(block.Labels[0]) {
			return nil, errors.E(
				hcl.ErrTerramateSchema,
				"first global label must be a valid identifier but got %s",
				block.Labels[0],
			)
		}

		if len(block.Labels) > 0 && len(block.Attributes) == 0 {
			stmts = append(stmts, Stmt{
				Origin: Ref{
					Object: "global",
					Path:   block.Labels,
				},
				LHS: Ref{
					Object: "global",
					Path:   block.Labels,
				},
				Special: true,
				Scope:   tree.Dir(),
			})
			continue
		}

		for _, attr := range block.Attributes.SortedList() {
			origin := Ref{
				Object: "global",
				Path:   make([]string, len(block.Labels)+1),
			}
			copy(origin.Path, block.Labels)
			origin.Path[len(block.Labels)] = attr.Name
			blockStmts, err := g.stmtsOf(tree.Dir(), origin, origin.Path, attr.Expr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, blockStmts...)
		}
	}

	// bigger refs -> smaller refs
	sort.Slice(stmts, func(i, j int) bool {
		return len(stmts[i].Origin.Path) > len(stmts[j].Origin.Path)
	})

	g.Scopes[tree.Dir()] = stmts
	return stmts, nil
}

func (g *G) lookupStmts(ref Ref) (Stmts, error) {
	return g.lookupStmtsAt(ref, g.tree)
}

func (g *G) lookupStmtsAt(ref Ref, tree *config.Tree) (Stmts, error) {
	stmts, err := g.loadStmtsAt(tree)
	if err != nil {
		return nil, err
	}
	filtered, found := stmts.filter(ref)
	if found || tree.Parent == nil {
		return filtered, nil
	}
	parentStmts, err := g.lookupStmtsAt(ref, tree.Parent)
	if err != nil {
		return nil, err
	}
	filtered = append(filtered, parentStmts...)
	return filtered, nil
}

func (g *G) stmtsOf(scope project.Path, origin Ref, base []string, expr hhcl.Expression) (Stmts, error) {
	stmts := Stmts{}
	newbase := make([]string, len(base)+1, len(base)+1)
	copy(newbase, base)
	last := len(newbase) - 1
	switch e := expr.(type) {
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			val, err := g.Eval(item.KeyExpr)
			if err != nil {
				return nil, err
			}

			if !val.Type().Equals(cty.String) {
				panic(errors.E("unexpected key type %s", val.Type().FriendlyName()))
			}

			newbase[last] = val.AsString()
			newStmts, err := g.stmtsOf(scope, origin, newbase, item.ValueExpr)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, newStmts...)
		}
	default:
		lhs := Ref{
			Object: "global",
			Path:   newbase[0:last],
		}
		stmts = append(stmts, Stmt{
			Origin: origin,
			LHS:    lhs,
			RHS:    expr,
			Scope:  scope,
		})
	}

	return stmts, nil
}

func (g *G) set(ref Ref, val cty.Value) error {
	g.globals.byref[ref.AsKey()] = val
	// bykey
	obj := g.globals.bykey

	// len(path) >= 1

	lastIndex := len(ref.Path) - 1
	for _, path := range ref.Path[:lastIndex] {
		v, ok := obj.Get(path)
		if ok {
			switch vv := v.(type) {
			case *orderedmap.OrderedMap[string, any]:
				obj = vv
			case cty.Value:
				return errors.E("%s points to a %s type but expects an object", ref, vv.Type().FriendlyName())
			default:
				panic("unexpected")
			}
		} else {
			tempMap := orderedmap.New[string, any]()
			obj.Set(path, tempMap)
			obj = tempMap
		}
	}

	obj.Set(ref.Path[lastIndex], val)
	//g.globals.bykey = obj

	fmt.Printf("globals updated: %s = %s\n", ref, ast.TokensForValue(val).Bytes())
	return nil
}

func (stmts Stmts) filter(ref Ref) (Stmts, bool) {
	filtered := Stmts{}
	found := false
	for _, stmt := range stmts {
		fmt.Printf("checking if %s contains %s: ", stmt.LHS, ref)
		if stmt.LHS.has(ref) {
			fmt.Printf("yes\n")
			filtered = append(filtered, stmt)
			if stmt.Origin.equal(ref) || stmt.LHS.equal(ref) {
				found = true
			}
		} else {
			fmt.Printf("no\n")
			if found {
				break
			}
		}
	}
	return filtered, found
}

func (stmt Stmt) String() string {
	var rhs string
	if stmt.Special {
		rhs = "{}"
	} else {
		rhs = string(ast.TokensForExpression(stmt.RHS).Bytes())
	}
	return fmt.Sprintf("%s = %s (defined at %s)",
		stmt.LHS,
		rhs,
		stmt.Scope)
}

func tocty(globals *orderedmap.OrderedMap[string, any]) cty.Value {
	ret := map[string]cty.Value{}
	for pair := globals.Oldest(); pair != nil; pair = pair.Next() {
		switch vv := pair.Value.(type) {
		case *orderedmap.OrderedMap[string, any]:
			ret[pair.Key] = tocty(vv)
		case cty.Value:
			ret[pair.Key] = vv
		default:
			panic("unexpected")
		}
	}
	return cty.ObjectVal(ret)
}

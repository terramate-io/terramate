// Copyright 2023 Mineiros GmbH
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

package globals2

import (
	"fmt"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when parsing and evaluating globals.
const (
	ErrEval  errors.Kind = "global eval"
	ErrCycle errors.Kind = "cycle detected"
)

type (
	// G is the globals evaluator.
	G struct {
		ctx *eval.Context
		ns  namespaces

		evaluators map[string]StmtLookup
	}

	StmtLookup interface {
		Root() string
		LoadStmts() (Stmts, error)
		LookupRef(Ref) (Stmts, error)
	}

	namespaces map[string]globals

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

	// Stmts is a list of statements.
	Stmts []Stmt
)

// New globals evaluator.
// TODO(i4k): better document.
func New(ctx *eval.Context, evaluators ...StmtLookup) *G {
	g := &G{
		ctx:        ctx,
		evaluators: map[string]StmtLookup{},
		ns:         namespaces{},
	}

	for _, ev := range evaluators {
		g.evaluators[ev.Root()] = ev
		g.ns[ev.Root()] = globals{
			byref: make(map[RefStr]cty.Value),
			bykey: orderedmap.New[string, any](),
		}
	}

	return g
}

// Eval the given expr and all of its dependency references (if needed)
// using a bottom-up algorithm (starts in the target g.tree directory
// and lookup references in parent directories when needed).
// The algorithm is reliable but it does the minimum required work to
// get the expr evaluated, and then it does not validate all globals
// scopes but only the ones it traversed into.
func (g *G) Eval(expr hhcl.Expression) (cty.Value, error) {
	return g.eval(expr, map[RefStr]Ref{})
}

func (g *G) eval(expr hhcl.Expression, visited map[RefStr]Ref) (cty.Value, error) {
	refs := refsOf(expr)
	for _, dep := range refs {
		if _, ok := visited[dep.AsKey()]; ok {
			return cty.NilVal, errors.E(
				ErrCycle,
				expr.Range(),
				"globals have circular dependencies: reference %s already evaluated",
				dep,
			)

		}
		visited[dep.AsKey()] = dep
		if _, ok := g.ns.Get(dep); ok {
			// dep already evaluated.
			continue
		}

		stmtResolver, ok := g.evaluators[dep.Object]
		if !ok {
			return cty.NilVal, errors.E(
				ErrEval,
				"unknown variable namespace %s: evaluating %s",
				dep.Object, ast.TokensForExpression(expr).Bytes(),
			)
		}

		stmts, err := stmtResolver.LookupRef(dep)
		if err != nil {
			return cty.NilVal, err
		}
		if len(stmts) == 0 {
			return cty.NilVal, errors.E(
				ErrEval,
				"evaluating %s: no global declaration found for %s",
				ast.TokensForExpression(expr).Bytes(), dep)
		}
		for _, stmt := range stmts {
			if _, ok := g.ns.Get(stmt.LHS); ok {
				// stmt already evaluated.
				// This can happen when the current scope is overriding the parent
				// object but still the target expr is looking for the entire object
				// so we still have to ascent into parent scope and then the "already
				// overridden" refs show up here.
				continue
			}
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

				err = g.set(stmt.LHS, val)
				if err != nil {
					return cty.NilVal, errors.E(ErrEval, err)
				}
			}
		}
	}

	for nsname, object := range g.ns {
		g.ctx.SetNamespace(nsname, tocty(object.bykey).AsValueMap())
	}

	val, err := g.ctx.Eval(expr)
	if err != nil {
		return cty.NilVal, errors.E(err, ErrEval, "failed to evaluate: %s", ast.TokensForExpression(expr).Bytes())
	}
	return val, nil
}

func (g *G) set(ref Ref, val cty.Value) error {
	if _, ok := g.ns[ref.Object]; !ok {
		panic(errors.E(errors.ErrInternal, "there's no evaluator for namespace %q", ref.Object))
	}

	g.ns[ref.Object].byref[ref.AsKey()] = val

	obj := g.ns[ref.Object].bykey

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
				panic("TODO")
			}
		} else {
			tempMap := orderedmap.New[string, any]()
			obj.Set(path, tempMap)
			obj = tempMap
		}
	}

	obj.Set(ref.Path[lastIndex], val)
	return nil
}

func (stmts Stmts) SelectBy(ref Ref) (Stmts, bool) {
	filtered := Stmts{}
	found := false
	for _, stmt := range stmts {
		if stmt.LHS.has(ref) {
			filtered = append(filtered, stmt)
			if stmt.Origin.equal(ref) || stmt.LHS.equal(ref) {
				found = true
			}
		} else {
			if found {
				break
			}
		}
	}
	return filtered, found
}

// String representation of the statement.
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

func (ns namespaces) Get(ref Ref) (cty.Value, bool) {
	if v, ok := ns[ref.Object]; ok {
		if vv, ok := v.byref[ref.AsKey()]; ok {
			return vv, true
		}
	}
	return cty.NilVal, false
}

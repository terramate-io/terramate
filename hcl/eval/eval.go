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

package eval

import (
	"reflect"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	hhcl "github.com/hashicorp/hcl/v2"
)

// Errors returned when parsing and evaluating globals.
const (
	// ErrEval indicates a failure during the evaluation process
	ErrEval errors.Kind = "eval expression"

	// ErrCycle indicates there's a cycle in the variable declarations.
	ErrCycle errors.Kind = "cycle detected"

	// ErrRedefined indicates the variable was already defined in this scope.
	ErrRedefined errors.Kind = "variable redefined"
)

type (
	// Context is the variables evaluator.
	Context struct {
		hclctx *hhcl.EvalContext
		ns     namespaces

		evaluators map[string]Resolver
	}

	// Resolver resolves unknown variable references.
	Resolver interface {
		Name() string
		Prevalue() cty.Value
		LookupRef(Ref) (Stmts, error)
	}

	namespaces map[string]namespace

	value struct {
		stmt  Stmt
		value cty.Value
		info  Info
	}

	namespace struct {
		byref map[RefStr]value
		bykey *orderedmap.OrderedMap[string, any]

		persist bool // whether persistence into internal context is needed.
	}
)

var unset cty.Type

func init() {
	unset = cty.Capsule("unset", reflect.TypeOf(struct{}{}))
}

// New evaluator.
// TODO(i4k): better document.
func New(evaluators ...Resolver) *Context {
	hclctx := &hhcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}
	evalctx := &Context{
		hclctx:     hclctx,
		evaluators: map[string]Resolver{},
		ns:         namespaces{},
	}

	for _, ev := range evaluators {
		evalctx.SetResolver(ev)
	}

	unsetVal := cty.CapsuleVal(unset, &struct{}{})
	evalctx.hclctx.Variables["unset"] = unsetVal

	return evalctx
}

// SetResolver sets the resolver ev into the context.
func (c *Context) SetResolver(ev Resolver) {
	c.evaluators[ev.Name()] = ev
	ns := namespace{
		persist: true,
		byref:   make(map[RefStr]value),
		bykey:   orderedmap.New[string, any](),
	}
	c.ns[ev.Name()] = ns

	prevalue := ev.Prevalue()
	if prevalue.Type().IsObjectType() {
		values := prevalue.AsValueMap()
		for key, val := range values {
			err := c.set(Stmt{
				Origin: Ref{
					Object: ev.Name(),
					Path:   []string{key},
				},
				LHS: Ref{
					Object: ev.Name(),
					Path:   []string{key},
				},
			}, val)
			if err != nil {
				panic(errors.E(errors.ErrInternal, "failed to initialize context"))
			}
		}
	}

	c.hclctx.Variables[ev.Name()] = prevalue
}

// DeleteResolver removes the resolver.
func (c *Context) DeleteResolver(name string) {
	delete(c.evaluators, name)
}

// Eval the given expr and all of its dependency references (if needed)
// using a bottom-up algorithm (starts in the target g.tree directory
// and lookup references in parent directories when needed).
// The algorithm is reliable but it does the minimum required work to
// get the expr evaluated, and then it does not validate all globals
// scopes but only the ones it traversed into.
func (c *Context) Eval(expr hhcl.Expression) (cty.Value, error) {
	return c.eval(expr, map[RefStr]hhcl.Expression{})
}

func (c *Context) eval(expr hhcl.Expression, visited map[RefStr]hhcl.Expression) (cty.Value, error) {
	refs := refsOf(expr)
	unsetRefs := map[RefStr]bool{}

	for _, dep := range refs {
		if dep.String() == "unset" {
			continue
		}

		if _, ok := c.ns.Get(dep); ok {
			// dep already evaluated.
			continue
		}

		if originalExpr, ok := visited[dep.AsKey()]; ok {
			return cty.NilVal, errors.E(
				ErrCycle,
				expr.Range(),
				"variable have circular dependencies: "+
					"reference %s already evaluated in the expression %s",
				dep,
				ast.TokensForExpression(originalExpr).Bytes(),
			)
		}

		visited[dep.AsKey()] = expr

		stmtResolver, ok := c.evaluators[dep.Object]
		if !ok {
			// because tm_ternary
			continue
		}

		stmts, err := stmtResolver.LookupRef(dep)
		if err != nil {
			return cty.NilVal, err
		}

		for _, stmt := range stmts {
			if v, ok := c.ns.Get(stmt.LHS); ok {
				if !stmt.Special && !v.stmt.Special &&
					v.info.Scope == stmt.Info.Scope &&
					v.info.DefinedAt.Path().Dir() == stmt.Info.DefinedAt.Path().Dir() &&
					v.info.DefinedAt != stmt.Info.DefinedAt {
					return cty.NilVal, errors.E(ErrRedefined, stmt.Info.DefinedAt,
						"variable %s already set in the scope %s at %s",
						stmt, stmt.Info.Scope, v.info.DefinedAt.String())
				}
				if !v.value.Type().IsObjectType() || !v.stmt.Special {

					// stmt already evaluated
					// This can happen when the current scope is overriding the parent
					// object but still the target expr is looking for the entire object
					// so we still have to ascent into parent scope and then the "already
					// overridden" refs show up here.
					continue
				}
			}

			var val cty.Value
			var err error
			if stmt.Special {
				val = cty.ObjectVal(map[string]cty.Value{})
			} else {
				val, err = c.eval(stmt.RHS, visited)
			}

			if err != nil {
				return cty.NilVal, errors.E(err, "evaluating %s from %s scope", stmt.LHS, stmt.Info.Scope)
			}

			if val.Type().Equals(unset) {
				unsetRefs[stmt.LHS.AsKey()] = true
				continue
			}

			if unsetRefs[stmt.LHS.AsKey()] {
				continue
			}

			err = c.set(stmt, val)
			if err != nil {
				return cty.NilVal, errors.E(ErrEval, err)
			}
		}

		if _, ok := c.ns.Get(dep); !ok {
			delete(visited, dep.AsKey())
		}
	}

	for nsname, ns := range c.ns {
		if ns.persist {
			c.SetNamespace(nsname, tocty(ns.bykey).AsValueMap())
			ns.persist = false
		}
	}

	val, diags := expr.Value(c.hclctx)
	if diags.HasErrors() {
		return cty.NilVal, errors.E(ErrEval, diags)
	}
	return val, nil
}

func (c *Context) set(lhs Stmt, val cty.Value) error {
	ref := lhs.LHS

	if val.Type().IsObjectType() && !lhs.Special {
		origin := Ref{
			Object: ref.Object,
			Path:   make([]string, len(ref.Path)),
		}
		copy(origin.Path, ref.Path)
		tokens := ast.TokensForValue(val)
		expr, _ := ast.ParseExpression(string(tokens.Bytes()), `<eval>`)

		stmts, err := StmtsOf(lhs.Info, origin, origin.Path, expr)
		if err != nil {
			return err
		}
		for _, s := range stmts {
			v, _ := s.RHS.Value(nil)
			if other, ok := c.ns.Get(s.LHS); ok {
				if !s.Special && !other.stmt.Special &&
					other.info.Scope == s.Info.Scope &&
					other.info.DefinedAt.Path().Dir() == s.Info.DefinedAt.Path().Dir() &&
					other.info.DefinedAt != s.Info.DefinedAt {
					return errors.E(ErrRedefined, s.Info.DefinedAt,
						"variable %s already set in the scope %s at %s",
						s, s.Info.Scope, other.info.DefinedAt.String())
				}
				if !other.value.Type().IsObjectType() || !other.stmt.Special {
					// stmt already evaluated
					// This can happen when the current scope is overriding the parent
					// object but still the target expr is looking for the entire object
					// so we still have to ascent into parent scope and then the "already
					// overridden" refs show up here.
					continue
				}
			}

			err := c.set(s, v)
			if err != nil {
				return err
			}
		}
		if len(stmts) > 0 {
			return nil
		}
	}

	ns, ok := c.ns[ref.Object]
	if !ok {
		panic(errors.E(errors.ErrInternal, "there's no evaluator for namespace %q", ref.Object))
	}

	oldval, hasold := ns.byref[ref.AsKey()]
	if lhs.Special {
		if !hasold {
			for _, r := range ref.Comb() {
				if _, ok := ns.byref[r.AsKey()]; !ok {
					ns.byref[r.AsKey()] = value{
						stmt: Stmt{
							LHS:          r,
							Origin:       r,
							Special:      true,
							RHSEvaluated: cty.EmptyObjectVal,
						},
						value: cty.EmptyObjectVal,
						info:  lhs.Info,
					}
					ns.persist = true
				} else {
					break
				}
			}
		}
	} else {
		if hasold {
			if len(oldval.info.Scope.String()) > len(lhs.Info.Scope.String()) {
				return nil
			}
		}
		for _, r := range ref.Comb() {
			if _, ok := ns.byref[r.AsKey()]; !ok {
				ns.byref[r.AsKey()] = value{
					stmt: Stmt{
						LHS:          r,
						Origin:       r,
						Special:      true,
						RHSEvaluated: cty.EmptyObjectVal,
					},
					value: cty.EmptyObjectVal,
					info:  lhs.Info,
				}
				ns.persist = true
			} else {
				break
			}
		}
		ns.byref[ref.AsKey()] = value{
			stmt:  lhs,
			value: val,
			info:  lhs.Info,
		}
		ns.persist = true
	}

	obj := ns.bykey

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
				panic(vv)
			}
		} else {
			tempMap := orderedmap.New[string, any]()
			obj.Set(path, tempMap)
			obj = tempMap
		}
	}

	if lhs.Special {
		_, found := obj.Get(ref.Path[lastIndex])
		if found {
			return nil
		}
		tempMap := orderedmap.New[string, any]()
		obj.Set(ref.Path[lastIndex], tempMap)

		return nil
	}

	if hasold && oldval.stmt.Special && oldval.info.Scope == lhs.Info.Scope {
		return errors.E(
			ErrEval,
			"variable %s being extended but was previously evaluated as %s in the same scope",
			lhs.LHS, ast.TokensForValue(oldval.value).Bytes(),
		)
	}
	ns.persist = true
	obj.Set(ref.Path[lastIndex], val)
	return nil
}

func tocty(globals *orderedmap.OrderedMap[string, any]) cty.Value {
	ret := map[string]cty.Value{}
	for pair := globals.Oldest(); pair != nil; pair = pair.Next() {
		switch vv := pair.Value.(type) {
		case *orderedmap.OrderedMap[string, any]:
			ret[pair.Key] = tocty(vv)
		case cty.Value:
			if vv.Type().IsTupleType() {
				var items []cty.Value
				it := vv.ElementIterator()
				for it.Next() {
					_, elem := it.Element()
					if !elem.Type().Equals(unset) {
						items = append(items, elem)
					}
				}
				if len(items) == 0 {
					vv = cty.EmptyTupleVal
				} else {
					vv = cty.TupleVal(items)
				}
			}
			ret[pair.Key] = vv
		default:
			panic("unexpected")
		}
	}
	return cty.ObjectVal(ret)
}

func (ns namespaces) Get(ref Ref) (value, bool) {
	if v, ok := ns[ref.Object]; ok {
		if vv, ok := v.byref[ref.AsKey()]; ok {
			return vv, true
		}
	}
	return value{}, false
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.hclctx.Variables[name] = cty.ObjectVal(vals)
}

// SetFunction sets the function in the context.
func (c *Context) SetFunction(name string, fn function.Function) {
	c.hclctx.Functions[name] = fn
}

// SetFunctions sets the functions of the context.
func (c *Context) SetFunctions(funcs map[string]function.Function) {
	c.hclctx.Functions = funcs
}

// SetEnv sets the given environment on the env namespace of the evaluation context.
// environ must be on the same format as os.Environ().
func (c *Context) SetEnv(environ []string) {
	env := map[string]cty.Value{}
	for _, v := range environ {
		parsed := strings.Split(v, "=")
		env[parsed[0]] = cty.StringVal(parsed[1])
	}
	c.SetNamespace("env", env)
}

// DeleteNamespace deletes the namespace name from the context.
// If name is not in the context, it's a no-op.
func (c *Context) DeleteNamespace(name string) {
	delete(c.hclctx.Variables, name)
}

// HasNamespace returns true the evaluation context knows this namespace, false otherwise.
func (c *Context) HasNamespace(name string) bool {
	_, has := c.hclctx.Variables[name]
	return has
}

// PartialEval evaluates only the terramate variable expressions from the list
// of tokens, leaving all the rest as-is. It returns a modified list of tokens
// with  no reference to terramate namespaced variables (globals and terramate)
// and functions (tm_ prefixed functions).
func (c *Context) PartialEval(expr hhcl.Expression) (hhcl.Expression, error) {
	newexpr, err := c.partialEval(expr)
	if err != nil {
		return nil, errors.E(ErrPartial, err)
	}
	return newexpr, nil
}

// Copy the eval context.
func (c *Context) Copy() *Context {
	newctx := &hhcl.EvalContext{
		Variables: map[string]cty.Value{},
	}
	newctx.Functions = c.hclctx.Functions
	for k, v := range c.hclctx.Variables {
		newctx.Variables[k] = v
	}
	return NewContextFrom(newctx)
}

// Unwrap returns the internal hhcl.EvalContext.
func (c *Context) Unwrap() *hhcl.EvalContext {
	return c.hclctx
}

// NewContextFrom creates a new evaluator from the hashicorp EvalContext.
func NewContextFrom(ctx *hhcl.EvalContext) *Context {
	return &Context{
		hclctx: ctx,
	}
}

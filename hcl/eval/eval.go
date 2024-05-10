// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"reflect"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
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
		scope    project.Path
		Internal *hhcl.EvalContext
		ns       namespaces

		evaluators map[string]Resolver
	}

	// Resolver resolves unknown variable references.
	Resolver interface {
		Name() string
		Prevalue() cty.Value
		LookupRef(scope project.Path, ref Ref) ([]Stmts, error)
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
func New(scope project.Path, evaluators ...Resolver) *Context {
	hclctx := &hhcl.EvalContext{
		Functions: map[string]function.Function{},
		Variables: map[string]cty.Value{},
	}
	evalctx := &Context{
		scope:      scope,
		Internal:   hclctx,
		evaluators: map[string]Resolver{},
		ns:         namespaces{},
	}

	for _, ev := range evaluators {
		evalctx.SetResolver(ev)
	}

	unsetVal := cty.CapsuleVal(unset, &struct{}{})
	evalctx.Internal.Variables["unset"] = unsetVal

	return evalctx
}

// SetResolver sets the resolver ev into the context.
func (c *Context) SetResolver(ev Resolver) {
	c.evaluators[ev.Name()] = ev
	ns := newNamespace()
	c.ns[ev.Name()] = ns

	prevalue := ev.Prevalue()
	if prevalue.Type().IsObjectType() {
		values := prevalue.AsValueMap()
		for key, val := range values {
			origin := NewRef(ev.Name(), []string{key}...)
			err := c.set(NewValStmt(origin, val, newBuiltinInfo(c.scope)), val)
			if err != nil {
				panic(errors.E(errors.ErrInternal, "failed to initialize context"))
			}
		}
	} else {
		c.Internal.Variables[ev.Name()] = prevalue
	}
}

// DeleteResolver removes the resolver.
func (c *Context) DeleteResolver(name string) {
	delete(c.evaluators, name)
	delete(c.Internal.Variables, name)
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
	refs, refsMap := refsOf(expr)
	unsetRefs := map[RefStr]bool{}

	for _, dep := range refs {
		if dep.String() == "unset" {
			continue
		}

		if _, ok := c.ns.Get(dep); ok {
			// dep already evaluated.
			continue
		}

		if previousExpr, ok := visited[dep.AsKey()]; ok {
			return cty.NilVal, errors.E(
				ErrCycle,
				expr.Range(),
				"variable have circular dependencies: "+
					"reference %s already evaluated in the expression %s",
				dep,
				ast.TokensForExpression(previousExpr).Bytes(),
			)
		}

		visited[dep.AsKey()] = expr

		stmtResolver, ok := c.evaluators[dep.Object]
		if !ok {
			// ignore unknowns in partial expressions
			continue
		}

		scopeStmts, err := stmtResolver.LookupRef(c.scope, dep)
		if err != nil {
			return cty.NilVal, err
		}

		for _, stmts := range scopeStmts {
			for _, stmt := range stmts {
				val, hasVal, err := c.evalStmt(stmt, visited)
				if err != nil {
					return cty.NilVal, err
				}

				if !hasVal {
					continue
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
		}

		if _, ok := c.ns.Get(dep); !ok {
			delete(visited, dep.AsKey())
		}
	}

	for nsname, ns := range c.ns {
		if ns.persist {
			if _, ok := refsMap[nsname]; ok {
				c.SetNamespace(nsname, tocty(ns.bykey).AsValueMap())
				ns.persist = false
			}
		}
	}

	val, diags := expr.Value(c.Internal)
	if diags.HasErrors() {
		return cty.NilVal, errors.E(ErrEval, diags)
	}

	return val, nil
}

func (c *Context) evalStmt(stmt Stmt, visited map[RefStr]hhcl.Expression) (cty.Value, bool, error) {
	if v, ok := c.ns.Get(stmt.LHS); ok {
		if isRedefined(stmt, v) {
			return cty.NilVal, false, errors.E(ErrRedefined, stmt.Info.DefinedAt,
				"variable %s already set in the scope %s at %s",
				stmt, stmt.Info.Scope, v.info.DefinedAt.String())
		}
		if !v.value.Type().IsObjectType() || !v.stmt.Special {

			// stmt already evaluated
			// This can happen when the current scope is overriding the parent
			// object but still the target expr is looking for the entire object
			// so we still have to ascent into parent scope and then the "already
			// overridden" refs show up here.
			return cty.NilVal, false, nil
		}
	}

	if stmt.Special {
		err := c.setExtend(stmt)
		if err != nil {
			return cty.NilVal, false, errors.E(ErrEval, err)
		}
		return cty.NilVal, false, nil
	}

	var val cty.Value
	var err error
	if stmt.RHS.IsEvaluated {
		val = stmt.RHS.Value
	} else {
		val, err = c.eval(stmt.RHS.Expression, visited)
		if err != nil {
			return cty.NilVal, false, errors.E(err, "evaluating %s from %s scope", stmt.LHS, stmt.Info.Scope)
		}
	}
	return val, true, nil
}

func (c *Context) setExtend(stmt Stmt) error {
	ref := stmt.LHS
	ns, ok := c.ns[ref.Object]
	if !ok {
		panic(errors.E(errors.ErrInternal, "there's no evaluator for namespace %q", ref.Object))
	}

	obj, err := traverseObject(&ns, ref, stmt.Info)
	if err != nil {
		return err
	}

	_, found := obj.Get(ref.LastAccessor())
	if found {
		return nil
	}
	tempMap := orderedmap.New[string, any]()
	obj.Set(ref.LastAccessor(), tempMap)
	return nil
}

func (c *Context) set(stmt Stmt, val cty.Value) error {
	ref := stmt.LHS

	if val.Type().IsObjectType() {
		origin := ref
		stmts := StmtsOfValue(stmt.Info, origin, origin.Path, val)
		for _, s := range stmts {
			val, hasVal, err := c.evalStmt(s, map[RefStr]hhcl.Expression{})
			if err != nil {
				return err
			}
			if !hasVal {
				continue
			}

			err = c.set(s, val)
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

	if hasold && len(oldval.info.Scope.String()) > len(stmt.Info.Scope.String()) {
		return nil
	}

	ns.byref[ref.AsKey()] = value{
		stmt:  stmt,
		value: val,
		info:  stmt.Info,
	}

	ns.persist = true

	obj, err := traverseObject(&ns, ref, stmt.Info)
	if err != nil {
		return err
	}

	if hasold && oldval.stmt.Special && oldval.info.Scope == stmt.Info.Scope {
		return errors.E(
			ErrEval,
			"variable %s being extended but was previously evaluated as %s in the same scope",
			stmt.LHS, ast.TokensForValue(oldval.value).Bytes(),
		)
	}
	ns.persist = true
	obj.Set(ref.LastAccessor(), val)
	return nil
}

func traverseObject(ns *namespace, ref Ref, info Info) (*orderedmap.OrderedMap[string, any], error) {
	obj := ns.bykey

	// len(path) >= 1

	lastIndex := len(ref.Path) - 1
	for i, path := range ref.Path[:lastIndex] {
		v, ok := obj.Get(path)
		if ok {
			switch vv := v.(type) {
			case *orderedmap.OrderedMap[string, any]:
				obj = vv
			case cty.Value:
				return nil, errors.E("%s points to a %s type but expects an object", ref, vv.Type().FriendlyName())
			default:
				panic(vv)
			}
		} else {
			r := ref
			r.Path = r.Path[:i+1]
			ns.byref[r.AsKey()] = value{
				stmt:  NewExtendStmt(r, info),
				value: cty.EmptyObjectVal,
				info:  info,
			}

			tempMap := orderedmap.New[string, any]()
			obj.Set(path, tempMap)
			obj = tempMap
		}
	}
	return obj, nil
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
			panic(errors.E(errors.ErrInternal, "unexpected type %T", vv))
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
	c.Internal.Variables[name] = cty.ObjectVal(vals)
}

// SetFunction sets the function in the context.
func (c *Context) SetFunction(name string, fn function.Function) {
	c.Internal.Functions[name] = fn
}

// DeleteFunction deletes the given function from the context.
func (c *Context) DeleteFunction(name string) {
	delete(c.Internal.Functions, name)
}

// SetFunctions sets the functions of the context.
func (c *Context) SetFunctions(funcs map[string]function.Function) {
	c.Internal.Functions = funcs
}

// DeleteNamespace deletes the namespace name from the context.
// If name is not in the context, it's a no-op.
func (c *Context) DeleteNamespace(name string) {
	delete(c.Internal.Variables, name)
}

// HasNamespace returns true the evaluation context knows this namespace, false otherwise.
func (c *Context) HasNamespace(name string) bool {
	_, has := c.Internal.Variables[name]
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
	newctx.Functions = c.Internal.Functions
	for k, v := range c.Internal.Variables {
		newctx.Variables[k] = v
	}
	return NewContextFrom(newctx)
}

// Unwrap returns the internal hhcl.EvalContext.
func (c *Context) Unwrap() *hhcl.EvalContext {
	return c.Internal
}

// NewContextFrom creates a new evaluator from the hashicorp EvalContext.
func NewContextFrom(ctx *hhcl.EvalContext) *Context {
	return &Context{
		Internal: ctx,
	}
}

func newNamespace() namespace {
	return namespace{
		persist: true,
		byref:   make(map[RefStr]value),
		bykey:   orderedmap.New[string, any](),
	}
}

func isRedefined(new Stmt, v value) bool {
	return !new.Special && !v.stmt.Special &&
		v.info.Scope == new.Info.Scope &&
		v.info.DefinedAt.Path().Dir() == new.Info.DefinedAt.Path().Dir() &&
		v.info.DefinedAt != new.Info.DefinedAt
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ErrEval indicates a failure during the evaluation process
const ErrEval errors.Kind = "eval expression"

// Context is used to evaluate HCL code.
type Context struct {
	hclctx *hhcl.EvalContext
}

// NewContext creates a new HCL evaluation context.
// The basedir is the base directory used by any interpolation functions that
// accept filesystem paths as arguments.
// The basedir must be an absolute path to a directory.
func NewContext(funcs map[string]function.Function) *Context {
	hclctx := &hhcl.EvalContext{
		Functions: funcs,
		Variables: map[string]cty.Value{},
	}
	return &Context{
		hclctx: hclctx,
	}
}

// ChildContext creates a new child HCL context that inherits all parent HCL variables and functions.
func (c *Context) ChildContext() *Context {
	child := NewContext(c.hclctx.Functions)
	for k, v := range c.hclctx.Variables {
		child.hclctx.Variables[k] = v
	}
	return child
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.hclctx.Variables[name] = cty.ObjectVal(vals)
}

// SetNamespaceRaw set the given namespace name with the provided value, no
// matter what value type is it.
func (c *Context) SetNamespaceRaw(name string, val cty.Value) {
	c.hclctx.Variables[name] = val
}

// GetNamespace will retrieve the value for the given namespace.
func (c *Context) GetNamespace(name string) (cty.Value, bool) {
	val, ok := c.hclctx.Variables[name]
	return val, ok
}

// SetFunction sets the function in the context.
func (c *Context) SetFunction(name string, fn function.Function) {
	c.hclctx.Functions[name] = fn
}

// SetEnv sets the given environment on the env namespace of the evaluation context.
// environ must be on the same format as os.Environ().
func (c *Context) SetEnv(environ []string) {
	env := map[string]cty.Value{}
	for _, v := range environ {
		equalAt := strings.Index(v, "=") // must always find
		env[v[:equalAt]] = cty.StringVal(v[equalAt+1:])
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

// Eval will evaluate an expression given its context.
func (c *Context) Eval(expr hhcl.Expression) (cty.Value, error) {
	val, diag := expr.Value(c.hclctx)
	if diag.HasErrors() {
		return cty.NilVal, errors.E(ErrEval, diag)
	}
	return val, nil
}

// PartialEval evaluates only the terramate variable expressions from the list
// of tokens, leaving all the rest as-is. It returns a modified list of tokens
// with  no reference to terramate namespaced variables (global, let and terramate)
// and tm_ prefixed functions. It also returns a boolean that tells if the returned
// expression contains any unknown references (variables or functions).
// It try to reduce the expression to its simplest form, which means expressions
// with no unknowns are evaluated down to literals.
func (c *Context) PartialEval(expr hhcl.Expression) (hhcl.Expression, bool, error) {
	newexpr, hasUnknowns, err := c.partialEval(expr)
	if err != nil {
		return nil, false, errors.E(ErrPartial, err)
	}

	if hasUnknowns {
		return newexpr, hasUnknowns, nil
	}
	switch newexpr.(type) {
	case *hclsyntax.LiteralValueExpr, *hclsyntax.TemplateExpr, *hclsyntax.TemplateWrapExpr:
		// NOTE(i4k): Template*Expr are also kept because we support an special
		// HEREDOC handling detection and then if evaluated it will be converted
		// to plain quoted strings.
		return newexpr, hasUnknowns, nil
	}
	var evaluated cty.Value
	evaluated, err = c.Eval(newexpr)
	if err != nil {
		return nil, false, errors.E(ErrPartial, err)
	}
	newexpr = &hclsyntax.LiteralValueExpr{
		Val:      evaluated,
		SrcRange: newexpr.Range(),
	}
	return newexpr, hasUnknowns, nil
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

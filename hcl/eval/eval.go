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
	"io/ioutil"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"

	hhcl "github.com/hashicorp/hcl/v2"
	tflang "github.com/hashicorp/terraform/lang"
)

// ErrEval indicates a failure during the evaluation process
const ErrEval errors.Kind = "failed to evaluate expression"

// Context is used to evaluate HCL code.
type Context struct {
	hclctx *hhcl.EvalContext
}

// NewContext creates a new HCL evaluation context.
// basedir is the base directory used by any interpolation functions that
// accept filesystem paths as arguments.
func NewContext(basedir string) *Context {
	scope := &tflang.Scope{BaseDir: basedir}
	hclctx := &hhcl.EvalContext{
		Functions: newTmFunctions(scope.Functions()),
		Variables: map[string]cty.Value{},
	}
	return &Context{
		hclctx: hclctx,
	}
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.hclctx.Variables[name] = FromMapToObject(vals)
}

// HasNamespace returns true the evaluation context knows this namespace, false otherwise.
func (c *Context) HasNamespace(name string) bool {
	_, has := c.hclctx.Variables[name]
	return has
}

// Eval will evaluate an expression given its context.
func (c *Context) Eval(expr hclsyntax.Expression) (cty.Value, error) {
	val, diag := expr.Value(c.hclctx)
	if diag.HasErrors() {
		return cty.NilVal, errors.E(ErrEval, diag)
	}
	return val, nil
}

// PartialEval evaluates only the terramate variable expressions from the list
// of tokens, leaving all the rest as-is. It returns a modified list of tokens
// with  no reference to terramate namespaced variables (globals and terramate)
// and functions (tm_ prefixed functions).
func (c *Context) PartialEval(expr hclsyntax.Expression) (hclwrite.Tokens, error) {
	exprFname := expr.Range().Filename
	filedata, err := ioutil.ReadFile(exprFname)
	if err != nil {
		return nil, errors.E(err, "reading expression from file")
	}

	tokens, err := GetExpressionTokens(filedata, exprFname, expr)
	if err != nil {
		return nil, err
	}

	engine := newPartialEvalEngine(tokens, c)
	return engine.Eval()
}

// GetExpressionTokens gets the provided expression as writable tokens.
func GetExpressionTokens(hcldoc []byte, filename string, expr hclsyntax.Expression) (hclwrite.Tokens, error) {
	exprRange := expr.Range()
	exprBytes := hcldoc[exprRange.Start.Byte:exprRange.End.Byte]
	tokens, diags := hclsyntax.LexExpression(exprBytes, filename, hhcl.Pos{})
	if diags.HasErrors() {
		return nil, errors.E(diags, "failed to scan expression")
	}
	return toWriteTokens(tokens), nil
}

// FromMapToObject converts a map of cty.Value to an object.
// It is a programming error to pass any cty.Value that can't be
// converted properly into an object field.
func FromMapToObject(values map[string]cty.Value) cty.Value {
	logger := log.With().
		Str("action", "fromMapToObject()").
		Logger()

	logger.Trace().Msg("converting map to object")

	ctyTypes := map[string]cty.Type{}
	for key, value := range values {
		logger.Trace().
			Str("name", key).
			Str("type", value.Type().FriendlyName()).
			Msg("building key to type map")

		ctyTypes[key] = value.Type()
	}

	ctyObject := cty.Object(ctyTypes)
	ctyVal, err := gocty.ToCtyValue(values, ctyObject)
	if err != nil {
		panic(errors.E("bug: all map values should be convertable", err))
	}

	logger.Trace().Msg("converted values map to cty object")

	return ctyVal
}

func toWriteTokens(in hclsyntax.Tokens) hclwrite.Tokens {
	tokens := make([]*hclwrite.Token, len(in))
	for i, st := range in {
		tokens[i] = &hclwrite.Token{
			Type:  st.Type,
			Bytes: st.Bytes,
		}
	}
	return tokens
}

func newTmFunctions(tffuncs map[string]function.Function) map[string]function.Function {
	tmfuncs := map[string]function.Function{}
	for name, function := range tffuncs {
		tmfuncs["tm_"+name] = function
	}
	return tmfuncs
}

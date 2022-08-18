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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
)

// ErrEval indicates a failure during the evaluation process
const ErrEval errors.Kind = "failed to evaluate expression"

// Context is used to evaluate HCL code.
type Context struct {
	Hclctx *hhcl.EvalContext
}

// ExpressionStringMark is the type used for marking expression values with the
// expression string.
type ExpressionStringMark string

// NewContext creates a new HCL evaluation context.
// The basedir is the base directory used by any interpolation functions that
// accept filesystem paths as arguments.
// The basedir must be an absolute path to a directory.
func NewContext(basedir string) (*Context, error) {
	if !filepath.IsAbs(basedir) {
		panic(fmt.Errorf("context created with relative path: %q", basedir))
	}

	st, err := os.Stat(basedir)
	if err != nil {
		return nil, errors.E(err, "failed to stat context basedir %q", basedir)
	}
	if !st.IsDir() {
		return nil, errors.E("context basedir (%s) must be a directory", basedir)
	}

	hclctx := &hhcl.EvalContext{
		Functions: newTmFunctions(basedir),
		Variables: map[string]cty.Value{},
	}
	return &Context{
		Hclctx: hclctx,
	}, nil
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.Hclctx.Variables[name] = cty.ObjectVal(vals)
}

// DeleteNamespace deletes the namespace name from the context.
// If name is not in the context, it's a no-op.
func (c *Context) DeleteNamespace(name string) {
	delete(c.Hclctx.Variables, name)
}

// HasNamespace returns true the evaluation context knows this namespace, false otherwise.
func (c *Context) HasNamespace(name string) bool {
	_, has := c.Hclctx.Variables[name]
	return has
}

// Eval will evaluate an expression given its context.
func (c *Context) Eval(expr hclsyntax.Expression) (cty.Value, error) {
	val, diag := expr.Value(c.Hclctx)
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
	tokens, err := TokensForExpression(expr)
	if err != nil {
		return nil, err
	}

	engine := newPartialEvalEngine(tokens, c)
	return engine.Eval()
}

// TokensForValue returns the tokens for the provided value.
func TokensForValue(value cty.Value) (hclwrite.Tokens, error) {
	value, marks := value.Unmark()
	if value.Type() == customdecode.ExpressionClosureType {
		closureExpr := value.EncapsulatedValue().(*customdecode.ExpressionClosure)
		for m := range marks {
			switch v := m.(type) {
			case ExpressionStringMark:
				exprRange := closureExpr.Expression.Range()
				exprData := []byte(v)[exprRange.Start.Byte:exprRange.End.Byte]
				return TokensForExpressionBytes(exprData)
			}
		}

		return TokensForExpression(closureExpr.Expression)
	}
	return hclwrite.TokensForValue(value), nil
}

// TokensForExpression gets the provided expression writable tokens.
// Beware: This function requires a valid expr.Range(), which means that
// expr.Range().Filename must be the real file used to parse this expression
// and the ranges (start and end) points to correct positions.
// The expression must be an expression parsed from a real file.
func TokensForExpression(expr hhcl.Expression) (hclwrite.Tokens, error) {
	filename := expr.Range().Filename
	filedata, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.E(err, "reading expression from file")
	}
	exprRange := expr.Range()
	exprBytes := filedata[exprRange.Start.Byte:exprRange.End.Byte]
	return TokensForExpressionBytes(exprBytes)
}

// TokensForExpressionBytes returns the tokens for the provided expression bytes.
func TokensForExpressionBytes(exprBytes []byte) (hclwrite.Tokens, error) {
	tokens, diags := hclsyntax.LexExpression(exprBytes, "", hhcl.Pos{})
	if diags.HasErrors() {
		return nil, errors.E(diags, "failed to scan expression")
	}
	return toWriteTokens(tokens), nil
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

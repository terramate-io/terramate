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
	"strings"

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
	hclctx *hhcl.EvalContext
}

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
		hclctx: hclctx,
	}, nil
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.hclctx.Variables[name] = cty.ObjectVal(vals)
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
// with  no reference to terramate namespaced variables (globals and terramate)
// and functions (tm_ prefixed functions).
func (c *Context) PartialEval(expr hhcl.Expression) (hclwrite.Tokens, error) {
	tokens, err := TokensForExpression(expr)
	if err != nil {
		return nil, err
	}

	engine := newPartialEvalEngine(tokens, c)
	return engine.Eval()
}

// TokensForValue returns the tokens for the provided value.
func TokensForValue(value cty.Value) (hclwrite.Tokens, error) {
	if value.Type() == customdecode.ExpressionClosureType {
		closureExpr := value.EncapsulatedValue().(*customdecode.ExpressionClosure)
		return TokensForExpression(closureExpr.Expression)
	} else if value.Type() == customdecode.ExpressionType {
		return TokensForExpression(customdecode.ExpressionFromVal(value))
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
	var exprdata []byte
	if strings.HasPrefix(filename, injectedTokensPrefix) {
		exprdata = []byte(filename[len(injectedTokensPrefix):])
	} else {
		var err error
		exprdata, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, errors.E(err, "reading expression from file")
		}
	}
	exprRange := expr.Range()
	exprdata = exprdata[exprRange.Start.Byte:exprRange.End.Byte]
	return TokensForExpressionBytes(exprdata)
}

// TokensForExpressionBytes returns the tokens for the provided expression bytes.
func TokensForExpressionBytes(exprBytes []byte) (hclwrite.Tokens, error) {
	tokens, diags := hclsyntax.LexExpression(exprBytes, "", hhcl.Pos{
		Line:   1,
		Column: 1,
		Byte:   0,
	})
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

func parseExpressionBytes(exprBytes []byte) (hhcl.Expression, error) {
	fname := fmt.Sprintf("%s%s", injectedTokensPrefix, exprBytes)
	expr, diags := hclsyntax.ParseExpression(exprBytes, fname, hhcl.Pos{
		Line:   1,
		Column: 1,
		Byte:   0,
	})

	if diags.HasErrors() {
		return nil, errors.E(diags, "parsing expression bytes")
	}
	return expr, nil
}

const injectedTokensPrefix = "<generated-hcl>:"

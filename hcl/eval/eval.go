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
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
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

// NewExtContext creates a new HCL evaluation context that includes extended/experimental
// functions like tm_vendor.
// It has the same invariants as NewExtContext, only adds extra functionality.
// - rootdir : The project root dir as an absolute path on the host.
// - basedir : The basedir as an absolute path on the host, the same as defined on [NewContext].
// - vendordir: Where modules are vendored inside the project.
// - stream: event stream, one event is produced per function call if there are no errors.
func NewExtContext(
	rootdir string,
	basedir string,
	vendordir project.Path,
	stream chan<- TmVendorEvent,
) (*Context, error) {
	ctx, err := NewContext(basedir)
	if err != nil {
		return nil, err
	}
	ctx.hclctx.Functions["tm_vendor"] = tmVendor(rootdir, basedir, vendordir, stream)
	return ctx, nil
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) {
	c.hclctx.Variables[name] = cty.ObjectVal(vals)
}

// GetNamespace will retrieve the value for the given namespace.
func (c *Context) GetNamespace(name string) (cty.Value, bool) {
	val, ok := c.hclctx.Variables[name]
	return val, ok
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

// TokensForExpression returns the tokens for the provided expression.
//
// Beware: This function has hacks to circumvent limitations in the hashicorp
// library when it comes to generating unknown values.
// Because we cannot retrieve the tokens that makes an hcl.Expression, we have
// to read the file bytes again and slice the expression part using the
// expr.Range() info. This was the first hack.
// But this is not enough... as in the case of partial evaluating expressions,
// we rewrite the token stream and once a hcl.Expression is generated, the
// tokens are lost forever and returned into the hashicorp evaluator. Then, if
// it composes with functions like tm_ternary(), we end up with expressions
// that lack information about its own tokens...
// So the second hack is: in the case of generated expressions, there's no real
// file, then we store the original bytes inside the expr.Range().Filename
// string. The expressions with these injected tokens have the filename of the
// form:
//
//	<generated-hcl><NUL BYTE><tokens>
//
// See the ParseExpressionBytes() function for details of how bytes are injected.
//
// At this point you should be wondering: What happens if the user creates a
// a file with this format? The answer depends on the user's operating system,
// but for most of them, this is impossible because POSIX systems and Windows
// prohibits NUL bytes in filesystem paths.
//
// I'm sorry.
func TokensForExpression(expr hhcl.Expression) (hclwrite.Tokens, error) {
	fnameBytes := []byte(expr.Range().Filename)
	var exprdata []byte
	if bytes.HasPrefix(fnameBytes, injectedTokensPrefix()) {
		exprdata = []byte(fnameBytes[len(injectedTokensPrefix()):])
	} else {
		var err error
		exprdata, err = os.ReadFile(string(fnameBytes))
		if err != nil {
			return nil, errors.E(err, "reading expression from file")
		}
	}
	exprRange := expr.Range()
	exprdata = exprdata[exprRange.Start.Byte:exprRange.End.Byte]
	return TokensForExpressionBytes(exprdata)
}

// ParseExpressionBytes parses the exprBytes into a hcl.Expression and stores
// the original tokens inside the expression.Range().Filename. For details
// about this craziness, see the TokensForExpression() function.
func ParseExpressionBytes(exprBytes []byte) (hhcl.Expression, error) {
	fnameBytes := append(injectedTokensPrefix(), exprBytes...)
	expr, diags := hclsyntax.ParseExpression(
		exprBytes,
		string(fnameBytes),
		hhcl.Pos{
			Line:   1,
			Column: 1,
			Byte:   0,
		})

	if diags.HasErrors() {
		return nil, errors.E(diags, "parsing expression bytes")
	}
	return expr, nil
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

func injectedTokensPrefix() []byte {
	return append(
		[]byte("<generated-hcl>"),
		0,
	)
}

func newContextFrom(ctx *hhcl.EvalContext) *Context {
	return &Context{
		hclctx: ctx,
	}
}

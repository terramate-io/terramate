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

	"github.com/hashicorp/hcl/v2"
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

// GetHCLContext gets the evaluation context.
func (c *Context) GetHCLContext() *hhcl.EvalContext {
	return c.hclctx
}

// SetNamespace will set the given values inside the given namespace on the
// evaluation context.
func (c *Context) SetNamespace(name string, vals map[string]cty.Value) error {
	logger := log.With().
		Str("action", "SetNamespace()").
		Logger()

	logger.Trace().
		Msg("Convert from map to object.")
	obj, err := fromMapToObject(vals)
	if err != nil {
		return fmt.Errorf("setting namespace %q:%v", name, err)
	}
	c.hclctx.Variables[name] = obj
	return nil
}

// TODO(katcipis): we need to extract globals from root terramate.
// Then this can be used to extract duplication from genfile/genhcl newEvalCtx functions.

// SetTerramateNamespaces will set Terrramate default namespaces according to the
// provided globals and stack metadata.
//func (c *Context) SetTerramateNamespaces(sm stack.Metadata, globals stack.Globals) error {
//logger := log.With().
//Str("action", "Context.SetTerramateNamespaces()").
//Logger()

//logger.Trace().Msg("Add terramate namespace")

//err := c.SetNamespace("terramate", stack.MetaToCtyMap(sm))
//if err != nil {
//return errors.E(sm, err, "setting terramate namespace on eval context")
//}

//logger.Trace().Msg("Add global evaluation namespace.")

//if err := c.SetNamespace("global", globals.Attributes()); err != nil {
//return errors.E(sm, err, "setting global namespace on eval context")
//}

//return nil
//}

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

	exprRange := expr.Range()
	exprBytes := filedata[exprRange.Start.Byte:exprRange.End.Byte]
	tokens, diags := hclsyntax.LexExpression(exprBytes, exprFname, hcl.Pos{})
	if diags.HasErrors() {
		return nil, errors.E(diags, "failed to scan expression")
	}

	engine := newPartialEvalEngine(toWriteTokens(tokens), c)
	return engine.Eval()
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

func fromMapToObject(m map[string]cty.Value) (cty.Value, error) {
	logger := log.With().
		Str("action", "fromMapToObject()").
		Logger()

	logger.Trace().
		Msg("Range over map.")
	ctyTypes := map[string]cty.Type{}
	for key, value := range m {
		ctyTypes[key] = value.Type()
	}

	logger.Trace().
		Msg("Convert type and value to object.")
	ctyObject := cty.Object(ctyTypes)
	ctyVal, err := gocty.ToCtyValue(m, ctyObject)
	if err != nil {
		return cty.Value{}, err
	}
	return ctyVal, nil
}

func newTmFunctions(tffuncs map[string]function.Function) map[string]function.Function {
	tmfuncs := map[string]function.Function{}
	for name, function := range tffuncs {
		tmfuncs["tm_"+name] = function
	}
	return tmfuncs
}

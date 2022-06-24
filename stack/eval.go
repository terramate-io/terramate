// Copyright 2022 Mineiros GmbH
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

package stack

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

// EvalCtx represents the evaluation context of a stack.
type EvalCtx struct {
	evalctx *eval.Context
}

// NewEvalCtx creates a new stack evaluation context.
func NewEvalCtx(sm Metadata, globals Globals) *EvalCtx {
	evalctx := &EvalCtx{evalctx: eval.NewContext(sm.HostPath())}
	evalctx.SetMetadata(sm)
	evalctx.SetGlobals(globals)
	return evalctx
}

// SetGlobals sets the given globals on the stack evaluation context.
func (e *EvalCtx) SetGlobals(g Globals) {
	e.evalctx.SetNamespace("global", g.Attributes())
}

// SetMetadata sets the given metadata on the stack evaluation context.
func (e *EvalCtx) SetMetadata(sm Metadata) {
	e.evalctx.SetNamespace("terramate", metaToCtyMap(sm))
}

// SetEnv sets the given environment on the env namespace of the evaluation context.
// environ must be on the same format as os.Environ().
func (e *EvalCtx) SetEnv(environ []string) {
	env := map[string]cty.Value{}
	for _, v := range environ {
		parsed := strings.Split(v, "=")
		env[parsed[0]] = cty.StringVal(parsed[1])
	}
	e.evalctx.SetNamespace("env", env)
}

// Eval will evaluate an expression given its context.
func (e *EvalCtx) Eval(expr hclsyntax.Expression) (cty.Value, error) {
	return e.evalctx.Eval(expr)
}

// PartialEval will partially evaluate an expression given its context.
func (e *EvalCtx) PartialEval(expr hclsyntax.Expression) (hclwrite.Tokens, error) {
	return e.evalctx.PartialEval(expr)
}

// HasNamespace returns true the evaluation context knows this namespace, false otherwise.
func (e *EvalCtx) HasNamespace(name string) bool {
	return e.evalctx.HasNamespace(name)
}

func metaToCtyMap(m Metadata) map[string]cty.Value {
	path := eval.FromMapToObject(map[string]cty.Value{
		"absolute": cty.StringVal(m.Path()),
		"relative": cty.StringVal(m.RelPath()),
		"basename": cty.StringVal(m.PathBase()),
		"to_root":  cty.StringVal(m.RelPathToRoot()),
	})
	stack := eval.FromMapToObject(map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),
		"description": cty.StringVal(m.Desc()),
		"path":        path,
	})
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()), // DEPRECATED
		"path":        cty.StringVal(m.Path()), // DEPRECATED
		"description": cty.StringVal(m.Desc()), // DEPRECATED
		"stack":       stack,
	}
}

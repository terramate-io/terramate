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
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// EvalCtx represents the evaluation context of a stack.
type EvalCtx struct {
	evalctx *eval.Context
}

// NewEvalCtx creates a new stack evaluation context.
func NewEvalCtx(stackpath string, sm Metadata, globals Globals) (*EvalCtx, error) {
	logger := log.With().
		Str("action", "stack.NewEvalCtx()").
		Str("path", stackpath).
		Logger()

	evalctx := &EvalCtx{evalctx: eval.NewContext(stackpath)}

	logger.Trace().Msg("Add stack metadata evaluation namespace.")

	err := evalctx.SetMetadata(sm)
	if err != nil {
		return nil, errors.E(sm, err, "setting terramate namespace on eval context")
	}

	logger.Trace().Msg("Add global evaluation namespace.")

	if err := evalctx.SetGlobals(globals); err != nil {
		return nil, err
	}

	return evalctx, nil
}

// SetGlobals sets the given globals on the stack evaluation context.
func (e *EvalCtx) SetGlobals(g Globals) error {
	return e.evalctx.SetNamespace("global", g.Attributes())
}

// SetMetadata sets the given metadata on the stack evaluation context.
func (e *EvalCtx) SetMetadata(sm Metadata) error {
	return e.evalctx.SetNamespace("terramate", metaToCtyMap(sm))
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
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),
		"path":        cty.StringVal(m.Path()),
		"description": cty.StringVal(m.Desc()),
	}
}

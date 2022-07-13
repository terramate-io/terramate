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
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// EvalCtx represents the evaluation context of a stack.
type EvalCtx struct {
	evalctx *eval.Context
}

// NewEvalCtx creates a new stack evaluation context.
func NewEvalCtx(rootdir string, sm Metadata, globals Globals) *EvalCtx {
	evalctx, err := eval.NewContext(sm.HostPath())
	if err != nil {
		panic(err)
	}
	evalwrapper := &EvalCtx{evalctx: evalctx}
	evalwrapper.SetMetadata(rootdir, sm)
	evalwrapper.SetGlobals(globals)
	return evalwrapper
}

func (e *EvalCtx) SetVariable(name string, value cty.Value) {
	e.evalctx.SetVariable(name, value)
}

func (e *EvalCtx) DeleteVariable(name string) {
	e.evalctx.DeleteVariable(name)
}

// SetGlobals sets the given globals on the stack evaluation context.
func (e *EvalCtx) SetGlobals(g Globals) {
	e.evalctx.SetNamespace("global", g.Attributes())
}

// SetMetadata sets the given metadata on the stack evaluation context.
func (e *EvalCtx) SetMetadata(rootdir string, sm Metadata) {
	e.evalctx.SetNamespace("terramate", metaToCtyMap(rootdir, sm))
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
	return e.evalctx.HasVariable(name)
}

func metaToCtyMap(rootdir string, m Metadata) map[string]cty.Value {
	logger := log.With().
		Str("action", "stack.metaToCtyMap()").
		Str("root", rootdir).
		Logger()

	logger.Trace().Msg("creating stack metadata")

	stackpath := eval.FromMapToObject(map[string]cty.Value{
		"absolute": cty.StringVal(m.Path()),
		"relative": cty.StringVal(m.RelPath()),
		"basename": cty.StringVal(m.PathBase()),
		"to_root":  cty.StringVal(m.RelPathToRoot()),
	})
	stackMapVals := map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),
		"description": cty.StringVal(m.Desc()),
		"path":        stackpath,
	}
	if id, ok := m.ID(); ok {
		logger.Trace().
			Str("id", id).
			Msg("adding stack ID to metadata")
		stackMapVals["id"] = cty.StringVal(id)
	}
	stack := eval.FromMapToObject(stackMapVals)
	rootfs := eval.FromMapToObject(map[string]cty.Value{
		"absolute": cty.StringVal(rootdir),
		"basename": cty.StringVal(filepath.Base(rootdir)),
	})
	rootpath := eval.FromMapToObject(map[string]cty.Value{
		"fs": rootfs,
	})
	root := eval.FromMapToObject(map[string]cty.Value{
		"path": rootpath,
	})
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()), // DEPRECATED
		"path":        cty.StringVal(m.Path()), // DEPRECATED
		"description": cty.StringVal(m.Desc()), // DEPRECATED
		"root":        root,
		"stack":       stack,
	}
}

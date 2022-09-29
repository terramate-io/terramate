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

	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// EvalCtx represents the evaluation context of a stack.
type EvalCtx struct {
	*eval.Context
}

// NewEvalCtx creates a new stack evaluation context.
func NewEvalCtx(projmeta project.Metadata, sm Metadata, globals globals.Map) *EvalCtx {
	evalctx, err := eval.NewContext(sm.HostPath())
	if err != nil {
		panic(err)
	}
	evalwrapper := &EvalCtx{
		Context: evalctx,
	}
	evalwrapper.SetMetadata(projmeta, sm)
	evalwrapper.SetGlobals(globals)
	return evalwrapper
}

// SetGlobals sets the given globals on the stack evaluation context.
func (e *EvalCtx) SetGlobals(g globals.Map) {
	e.SetNamespace("global", g.Attributes())
}

// SetMetadata sets the given metadata on the stack evaluation context.
func (e *EvalCtx) SetMetadata(projmeta project.Metadata, sm Metadata) {
	e.SetNamespace("terramate", MetadataToCtyValues(projmeta, sm))
}

// SetEnv sets the given environment on the env namespace of the evaluation context.
// environ must be on the same format as os.Environ().
func (e *EvalCtx) SetEnv(environ []string) {
	env := map[string]cty.Value{}
	for _, v := range environ {
		parsed := strings.Split(v, "=")
		env[parsed[0]] = cty.StringVal(parsed[1])
	}
	e.SetNamespace("env", env)
}

func stackMetaToCtyMap(m Metadata) map[string]cty.Value {
	logger := log.With().
		Str("action", "stack.stackMetaToCtyMap()").
		Logger()

	logger.Trace().Msg("creating stack metadata")

	stackpath := cty.ObjectVal(map[string]cty.Value{
		"absolute": cty.StringVal(m.Path().String()),
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
	stack := cty.ObjectVal(stackMapVals)
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),          // DEPRECATED
		"path":        cty.StringVal(m.Path().String()), // DEPRECATED
		"description": cty.StringVal(m.Desc()),          // DEPRECATED
		"stack":       stack,
	}
}

// MetadataToCtyValues converts the metadatas to a map of cty.Values.
func MetadataToCtyValues(projmeta project.Metadata, sm Metadata) map[string]cty.Value {
	projvalues := projmeta.ToCtyMap()
	stackvalues := stackMetaToCtyMap(sm)

	tmvar := map[string]cty.Value{}
	for k, v := range projvalues {
		tmvar[k] = v
	}
	for k, v := range stackvalues {
		if _, ok := tmvar[k]; ok {
			panic("project metadata and stack metadata conflicts")
		}
		tmvar[k] = v
	}
	return tmvar
}

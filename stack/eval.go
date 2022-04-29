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
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// NewEvalCtx creates a new evaluation context for a stack
func NewEvalCtx(stackpath string, sm Metadata, globals Globals) (*eval.Context, error) {
	logger := log.With().
		Str("action", "stack.NewEvalCtx()").
		Str("path", stackpath).
		Logger()

	evalctx := eval.NewContext(stackpath)

	logger.Trace().Msg("Add stack metadata evaluation namespace.")

	err := evalctx.SetNamespace("terramate", metaToCtyMap(sm))
	if err != nil {
		return nil, errors.E(sm, err, "setting terramate namespace on eval context")
	}

	logger.Trace().Msg("Add global evaluation namespace.")

	if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
		return nil, errors.E(sm, err, "setting global namespace on eval context")
	}

	return evalctx, nil
}

func metaToCtyMap(m Metadata) map[string]cty.Value {
	return map[string]cty.Value{
		"name":        cty.StringVal(m.Name()),
		"path":        cty.StringVal(m.Path()),
		"description": cty.StringVal(m.Desc()),
	}
}

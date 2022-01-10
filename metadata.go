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

package terramate

import (
	"strings"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// StackMetadata has all metadata loaded per stack
type StackMetadata struct {
	Name        string
	Path        string
	Description string
}

// Metadata has all metadata loader per project
type Metadata struct {
	// Stacks is a lexycographicaly sorted (by stack path) list of stack metadata
	Stacks []StackMetadata

	basedir string
}

// LoadMetadata loads the project metadata given the project basedir.
func LoadMetadata(basedir string) (Metadata, error) {
	logger := log.With().
		Str("action", "LoadMetadata()").
		Str("path", basedir).
		Logger()

	logger.Debug().
		Msg("Get list of stacks in path.")
	stackEntries, err := ListStacks(basedir)
	if err != nil {
		return Metadata{}, err
	}

	logger.Trace().
		Msg("Make array of stack metadata entries.")
	stacksMetadata := make([]StackMetadata, len(stackEntries))
	for i, stackEntry := range stackEntries {
		stacksMetadata[i] = StackMetadata{
			Name:        stackEntry.Stack.Name(),
			Description: stackEntry.Stack.Description(),
			Path:        strings.TrimPrefix(stackEntry.Stack.Dir, basedir),
		}
	}

	return Metadata{
		Stacks:  stacksMetadata,
		basedir: basedir,
	}, nil
}

// SetOnEvalCtx will add the proper namespace for evaluation of stack metadata
// on the given evaluation context.
func (m StackMetadata) SetOnEvalCtx(evalctx *eval.Context) error {
	// Not 100% sure this eval related logic should be here.
	vals := map[string]cty.Value{
		"name": cty.StringVal(m.Name),
		"path": cty.StringVal(m.Path),
	}
	return evalctx.SetNamespace("terramate", vals)
}

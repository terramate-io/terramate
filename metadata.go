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
	"fmt"
	"strings"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stack"
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
func LoadMetadata(root string) (Metadata, error) {
	logger := log.With().
		Str("action", "LoadMetadata()").
		Str("root", root).
		Logger()

	logger.Debug().
		Msg("Get list of stacks in path.")
	stackEntries, err := ListStacks(root)
	if err != nil {
		return Metadata{}, err
	}

	logger.Trace().
		Msg("Make array of stack metadata entries.")
	stacksMetadata := make([]StackMetadata, len(stackEntries))
	for i, stackEntry := range stackEntries {
		stacksMetadata[i] = newStackMetadata(root, stackEntry.Stack)
	}

	return Metadata{
		Stacks:  stacksMetadata,
		basedir: root,
	}, nil
}

// LoadStackMetadata loads the metadata for a specific stack.
func LoadStackMetadata(root string, stackDir string) (StackMetadata, error) {
	logger := log.With().
		Str("action", "LoadStackMetadata()").
		Str("root", root).
		Str("stackDir", stackDir).
		Logger()

	logger.Trace().Msg("loading stack metadata.")
	stackEntry, err := stack.Load(root, stackDir)
	if err != nil {
		return StackMetadata{}, fmt.Errorf("loading stack metadata: %v", err)
	}
	return newStackMetadata(root, stackEntry), nil
}

func newStackMetadata(root string, s stack.S) StackMetadata {
	return StackMetadata{
		Name:        s.Name(),
		Description: s.Description(),
		Path:        strings.TrimPrefix(s.Dir, root),
	}
}

// SetOnEvalCtx will add the proper namespace for evaluation of stack metadata
// on the given evaluation context.
func (m StackMetadata) SetOnEvalCtx(evalctx *eval.Context) error {
	// Not 100% sure this eval related logic should be here.
	vals := map[string]cty.Value{
		"name":        cty.StringVal(m.Name),
		"path":        cty.StringVal(m.Path),
		"description": cty.StringVal(m.Description),
	}
	return evalctx.SetNamespace("terramate", vals)
}

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

	tfhcl "github.com/hashicorp/hcl/v2"
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// StackMetadata has all metadata loaded per stack
type StackMetadata struct {
	Name string
	Path string
}

// Metadata has all metadata loader per project
type Metadata struct {
	// Stacks is a lexycographicaly sorted (by stack path) list of stack metadata
	Stacks []StackMetadata

	basedir string
}

// LoadMetadata loads the project metadata given the project basedir.
func LoadMetadata(basedir string) (Metadata, error) {
	stackEntries, err := ListStacks(basedir)
	if err != nil {
		return Metadata{}, err
	}

	stacksMetadata := make([]StackMetadata, len(stackEntries))
	for i, stackEntry := range stackEntries {
		stacksMetadata[i] = StackMetadata{
			Name: stackEntry.Stack.Name(),
			Path: strings.TrimPrefix(stackEntry.Stack.Dir, basedir),
		}
	}

	return Metadata{
		Stacks:  stacksMetadata,
		basedir: basedir,
	}, nil
}

// StackMetadata gets the metadata of a specific stack given its absolute path.
func (m Metadata) StackMetadata(abspath string) (StackMetadata, bool) {
	path := strings.TrimPrefix(abspath, m.basedir)
	for _, stackMetadata := range m.Stacks {
		if stackMetadata.Path == path {
			return stackMetadata, true
		}
	}
	return StackMetadata{}, false
}

func newHCLEvalContext(metadata StackMetadata, scope *tflang.Scope) (*tfhcl.EvalContext, error) {
	// TODO(katcipis): move to hcl package
	vars, err := hclMapToCty(map[string]cty.Value{
		"name": cty.StringVal(metadata.Name),
		"path": cty.StringVal(metadata.Path),
	})

	if err != nil {
		return nil, err
	}

	return &tfhcl.EvalContext{
		Variables: map[string]cty.Value{"terramate": vars},
		Functions: scope.Functions(),
	}, nil
}

func hclMapToCty(m map[string]cty.Value) (cty.Value, error) {
	// TODO(katcipis): move to hcl package
	ctyTypes := map[string]cty.Type{}
	for key, value := range m {
		ctyTypes[key] = value.Type()
	}
	ctyObject := cty.Object(ctyTypes)
	ctyVal, err := gocty.ToCtyValue(m, ctyObject)
	if err != nil {
		return cty.Value{}, err
	}
	return ctyVal, nil
}

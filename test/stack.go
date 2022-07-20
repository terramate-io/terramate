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

package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
)

// AssertStackImports checks that the given stack has all the wanted import
// definitions. The wanted imports is a slice of the sources that are imported
// on each block. The got stack must be not nil.
func AssertStackImports(t *testing.T, rootdir string, stackHostPath string, want []string) {
	t.Helper()

	parser, err := hcl.NewTerramateParser(rootdir, stackHostPath)
	assert.NoError(t, err)

	err = parser.AddDir(stackHostPath)
	assert.NoError(t, err)

	err = parser.Parse()
	assert.NoError(t, err)

	imports, err := parser.Imports()
	assert.NoError(t, err)

	if len(imports) != len(want) {
		t.Fatalf("got %d imports, wanted %v", len(imports), want)
	}

checkImports:
	for _, wantImport := range want {
		for _, gotImportBlock := range imports {
			sourceVal, diags := gotImportBlock.Attributes["source"].Expr.Value(nil)
			if diags.HasErrors() {
				t.Fatalf("error %v evaluating import source attribute", diags)
			}
			if sourceVal.AsString() == wantImport {
				continue checkImports
			}
		}
		t.Errorf("wanted import %s not found", wantImport)
	}
}

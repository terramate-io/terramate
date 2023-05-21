// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl"
)

// AssertStackImports checks that the given stack has all the wanted import
// definitions. The wanted imports is a slice of the sources that are imported
// on each block.
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

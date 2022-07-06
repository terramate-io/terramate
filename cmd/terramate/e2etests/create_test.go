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

package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCreateStack(t *testing.T) {
	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())

	const (
		stackID          = "stack-id"
		stackName        = "stack name"
		stackDescription = "stack description"
		stackImport1     = "/core/file1.tm.hcl"
		stackImport2     = "/core/file2.tm.hcl"
	)

	createFile := func(path string) {
		abspath := filepath.Join(s.RootDir(), path)
		test.WriteFile(t, filepath.Dir(abspath), filepath.Base(abspath), "")
	}

	createFile(stackImport1)
	createFile(stackImport2)

	stackPaths := []string{
		"stack-1",
		"/stack-2",
		"/stacks/stack-a",
		"stacks/stack-b",
	}

	for _, stackPath := range stackPaths {
		res := cli.run("create", stackPath,
			"--id", stackID,
			"--name", stackName,
			"--description", stackDescription,
			"--import", stackImport1,
			"--import", stackImport2,
		)

		assertRun(t, res)

		got := s.LoadStack(stackPath)

		gotID, _ := got.ID()
		assert.EqualStrings(t, stackID, gotID)
		assert.EqualStrings(t, stackName, got.Name(), "checking stack name")
		assert.EqualStrings(t, stackDescription, got.Desc(), "checking stack description")

		// TODO(katcipis): extract this to avoid duplication with stack.Create tests
		assertStackImports(t, s.RootDir(), got, []string{stackImport1, stackImport2})
	}
}

func TestCreateStackDefaults(t *testing.T) {
	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	cli.run("create", "stack")

	got := s.LoadStack("stack")

	assert.EqualStrings(t, "stack", got.Name(), "checking stack name")
	assert.EqualStrings(t, "stack", got.Desc(), "checking stack description")

	// TODO(katcipis): extract this to avoid duplication with stack.Create tests
	assertStackImports(t, s.RootDir(), got, []string{})
}

func assertStackImports(t *testing.T, rootdir string, got stack.S, want []string) {
	// TODO(katcipis): extract this to avoid duplication with stack.Create tests
	t.Helper()

	parser, err := hcl.NewTerramateParser(rootdir, got.HostPath())
	assert.NoError(t, err)

	err = parser.AddDir(got.HostPath())
	assert.NoError(t, err)

	err = parser.MinimalParse()
	assert.NoError(t, err)

	// TODO(katcipis): extract this to avoid duplication with stack.Create tests
	imports := ast.Blocks{}
	//imports, err := parser.Imports()
	//assert.NoError(t, err)

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

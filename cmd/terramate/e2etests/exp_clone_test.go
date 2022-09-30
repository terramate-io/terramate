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
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCloneStack(t *testing.T) {
	t.Parallel()

	const (
		srcStack         = "stack"
		destStack        = "cloned-stack"
		stackID          = "stack-id"
		stackName        = "stack name"
		stackDesc        = "stack description"
		stackCfgFilename = "stack.tm.hcl"
		stackCfgTemplate = `
// Commenting generate_hcl 1
generate_hcl "test.hcl" {
  content {
    // Commenting literal
    a = "literal"
  }
}
// Some comments
/*
  Commenting is fun
*/
stack {
  // Commenting stack ID
  id = %q // comment after ID expression
  // Commenting stack name
  name = %q // More comments !!
  // Commenting stack description
  description = %q
}
generate_hcl "test2.hcl" {
  content {
    b = tm_try(global.expression, null)
  }
}
`
	)
	s := sandbox.New(t)
	s.BuildTree([]string{"d:stack"})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateFile(stackCfgFilename, fmt.Sprintf(stackCfgTemplate,
		stackID, stackName, stackDesc))

	tmcli := newCLI(t, s.RootDir())
	res := tmcli.run("experimental", "clone", srcStack, destStack)

	assertRunResult(t, res, runExpected{
		StdoutRegex: fmt.Sprintf("Cloned stack %s to %s with success\n", srcStack, destStack),
	})

	destdir := filepath.Join(s.RootDir(), destStack)
	cfg := test.ParseTerramateConfig(t, destdir)

	if cfg.Stack == nil {
		t.Fatalf("cloned stack has no stack block: %v", cfg)
	}

	clonedStackID, ok := cfg.Stack.ID.Value()
	if !ok {
		t.Fatalf("cloned stack has no ID: %v", cfg.Stack)
	}

	if clonedStackID == stackID {
		t.Fatalf("want cloned stack to have different ID, got %s == %s", clonedStackID, stackID)
	}

	assert.EqualStrings(t, stackName, cfg.Stack.Name)
	assert.EqualStrings(t, stackDesc, cfg.Stack.Description)

	want := fmt.Sprintf(stackCfgTemplate, clonedStackID, stackName, stackDesc)

	clonedStackEntry := s.DirEntry(destStack)
	got := string(clonedStackEntry.ReadFile(stackCfgFilename))

	assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)

	// Checking that code was also generated already
	genHCL := string(clonedStackEntry.ReadFile("test.hcl"))
	genHCL2 := string(clonedStackEntry.ReadFile("test2.hcl"))

	test.AssertGenCodeEquals(t, genHCL, `a = "literal"`)
	test.AssertGenCodeEquals(t, genHCL2, `b = null`)
}

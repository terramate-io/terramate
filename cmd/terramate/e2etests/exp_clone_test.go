// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
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
	s := sandbox.NoGit(t, true)
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
	if cfg.Stack.ID == "" {
		t.Fatalf("cloned stack has no ID: %v", cfg.Stack)
	}
	if cfg.Stack.ID == stackID {
		t.Fatalf("want cloned stack to have different ID, got %s == %s", cfg.Stack.ID, stackID)
	}

	assert.EqualStrings(t, stackName, cfg.Stack.Name)
	assert.EqualStrings(t, stackDesc, cfg.Stack.Description)

	want := fmt.Sprintf(stackCfgTemplate, cfg.Stack.ID, stackName, stackDesc)

	clonedStackEntry := s.DirEntry(destStack)
	got := string(clonedStackEntry.ReadFile(stackCfgFilename))

	assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)

	// Checking that code was also generated already
	genHCL := string(clonedStackEntry.ReadFile("test.hcl"))
	genHCL2 := string(clonedStackEntry.ReadFile("test2.hcl"))

	test.AssertGenCodeEquals(t, genHCL, `a = "literal"`)
	test.AssertGenCodeEquals(t, genHCL2, `b = null`)
}

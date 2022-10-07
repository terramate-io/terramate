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

package generate_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCheckStackForGenHCLWithChildStacks(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:/stack",
		"s:/stack/dir/child",
	})

	assertEqualStringList(t, s.CheckStack("stack"), []string{})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{})

	stackEntry := s.DirEntry("stack")
	stackEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("test.tf"),
				Content(
					Terraform(
						Str("required_version", "1.10"),
					),
				),
			),
			GenerateHCL(
				Labels("dir/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
		).String())

	assertEqualStringList(t, s.CheckStack("stack"), []string{
		"dir/test.tf",
		"test.tf",
	})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{
		"dir/test.tf",
		"test.tf",
	})

	childEntry := s.DirEntry("stack/dir/child")
	childEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("another.tf"),
				Content(
					Terraform(
						Str("required_version", "1.10"),
					),
				),
			),
			GenerateHCL(
				Labels("another/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
		).String())

	assertEqualStringList(t, s.CheckStack("stack"), []string{
		"dir/test.tf",
		"test.tf",
	})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{
		"another.tf",
		"another/test.tf",
		"dir/test.tf",
		"test.tf",
	})

	s.Generate()

	assertEqualStringList(t, s.CheckStack("stack"), []string{})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{})

	// Removing configs makes all generated files outdated.
	// Then the outdated files are removed by generate.
	stackEntry.DeleteConfig()
	childEntry.DeleteConfig()

	assertEqualStringList(t, s.CheckStack("stack"), []string{
		"dir/test.tf",
		"test.tf",
	})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{
		"another.tf",
		"another/test.tf",
		"dir/test.tf",
		"test.tf",
	})

	s.Generate()

	assertEqualStringList(t, s.CheckStack("stack"), []string{})
	assertEqualStringList(t, s.CheckStack("stack/dir/child"), []string{})
}

func TestCheckStackForGenHCL(t *testing.T) {
	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.LoadProjectMetadata(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	// Checking detection when there is no config generated yet
	assertOutdatedFiles([]string{})
	stackEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("test.tf"),
				Content(
					Terraform(
						Str("required_version", "1.10"),
					),
				),
			),
			GenerateHCL(
				Labels("dir/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
			GenerateHCL(
				Labels("dir/sub/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"dir/sub/test.tf", "dir/test.tf", "test.tf"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Now checking when we have code + it gets outdated.
	stackEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("test.tf"),
				Content(
					Terraform(
						Str("required_version", "1.11"),
					),
				),
			),
			GenerateHCL(
				Labels("dir/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
			GenerateHCL(
				Labels("dir/sub/test.tf"),
				Content(
					Str("data", "new data"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"dir/sub/test.tf", "test.tf"})

	s.Generate()

	// Changing generated filenames will trigger detection,
	// with new + old filenames.
	stackEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("testnew.tf"),
				Content(
					Terraform(
						Str("required_version", "1.11"),
					),
				),
			),
			GenerateHCL(
				Labels("dir/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
			GenerateHCL(
				Labels("dir/sub/test.tf"),
				Content(
					Str("data", "new data"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"test.tf", "testnew.tf"})

	s.Generate()

	// Adding new filename to generation trigger detection
	stackEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("testnew.tf"),
				Content(
					Terraform(
						Str("required_version", "1.11"),
					),
				),
			),
			GenerateHCL(
				Labels("dir/test.tf"),
				Content(
					Str("data", "data"),
				),
			),
			GenerateHCL(
				Labels("dir/sub/test.tf"),
				Content(
					Str("data", "new data"),
				),
			),
			GenerateHCL(
				Labels("another.tf"),
				Content(
					Backend(
						Labels("type"),
					),
				),
			),
		).String())

	assertOutdatedFiles([]string{"another.tf"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Detects configurations that have been removed.
	stackEntry.DeleteConfig()

	assertOutdatedFiles([]string{
		"another.tf",
		"dir/sub/test.tf",
		"dir/test.tf",
		"testnew.tf",
	})

	s.Generate()

	assertOutdatedFiles([]string{})
}

func TestCheckOutdatedDetectsEmptyGenerateHCLBlocks(t *testing.T) {
	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.LoadProjectMetadata(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	stackEntry.CreateConfig(
		GenerateHCL(
			Labels("test.tf"),
			Content(),
		).String())

	assertOutdatedFiles([]string{"test.tf"})
	s.Generate()
	assertOutdatedFiles([]string{})

	// Check having generated code and switch to no code
	stackEntry.CreateConfig(
		GenerateHCL(
			Labels("test.tf"),
			Content(
				Str("test", "test"),
			),
		).String())

	assertOutdatedFiles([]string{"test.tf"})
	s.Generate()
	assertOutdatedFiles([]string{})

	stackEntry.CreateConfig(
		GenerateHCL(
			Labels("test.tf"),
			Content(),
		).String())

	assertOutdatedFiles([]string{"test.tf"})
	s.Generate()
	assertOutdatedFiles([]string{})
}

func TestCheckOutdatedIgnoresWhenGenHCLConditionIsFalse(t *testing.T) {
	const filename = "test.tf"

	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.LoadProjectMetadata(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	createConfig := func(filename string, condition bool) {
		stackEntry.CreateConfig(
			GenerateHCL(
				Labels(filename),
				Bool("condition", condition),
				Content(
					Block("whatever"),
				),
			).String())
	}

	// Checking detection when the condition is false
	createConfig(filename, false)
	assertOutdatedFiles([]string{})

	// Checking detection when the condition is true
	createConfig(filename, true)
	assertOutdatedFiles([]string{filename})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Checking the condition back to false triggers detection
	createConfig(filename, false)
	assertOutdatedFiles([]string{filename})

	s.Generate()

	assertOutdatedFiles([]string{})
}

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
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestCheckReturnsOutdatedStackFilenamesForGeneratedFile(t *testing.T) {
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
	stackEntry.CreateConfig(
		GenerateFile(
			Labels("test.txt"),
			Str("content", "test"),
		).String(),
	)
	assertOutdatedFiles([]string{"test.txt"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Now checking when we have code + it gets outdated.
	stackEntry.CreateConfig(
		GenerateFile(
			Labels("test.txt"),
			Str("content", "changed"),
		).String(),
	)

	assertOutdatedFiles([]string{"test.txt"})

	s.Generate()

	// Changing generated filenames will NOT trigger detection for the old file
	// since there is no way to automatically track the files for now
	stackEntry.CreateConfig(
		GenerateFile(
			Labels("testnew.txt"),
			Str("content", "changed"),
		).String(),
	)

	assertOutdatedFiles([]string{"testnew.txt"})

	// Adding new filename to generation trigger detection
	stackEntry.CreateConfig(
		Doc(
			GenerateFile(
				Labels("testnew.txt"),
				Str("content", "changed"),
			),
			GenerateFile(
				Labels("another.txt"),
				Str("content", "changed"),
			),
		).String())

	assertOutdatedFiles([]string{"another.txt", "testnew.txt"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Removed configurations will not be detected by default since there
	// is no way to track the files for now.
	stackEntry.DeleteConfig()

	assertOutdatedFiles([]string{})
}

func TestCheckOutdatedDetectsEmptyGenerateFileContent(t *testing.T) {
	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.LoadProjectMetadata(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	// Checking detection when the config is empty at first
	stackEntry.CreateConfig(
		GenerateFile(
			Labels("test.txt"),
			Str("content", ""),
		).String(),
	)

	assertOutdatedFiles([]string{"test.txt"})
	s.Generate()
	assertOutdatedFiles([]string{})

	// Check having generated code and switch to no code
	stackEntry.CreateConfig(
		GenerateFile(
			Labels("test.txt"),
			Str("content", "code"),
		).String(),
	)

	assertOutdatedFiles([]string{"test.txt"})
	s.Generate()
	assertOutdatedFiles([]string{})

	stackEntry.CreateConfig(
		GenerateFile(
			Labels("test.txt"),
			Str("content", ""),
		).String(),
	)

	assertOutdatedFiles([]string{"test.txt"})
	s.Generate()
	assertOutdatedFiles([]string{})
}

func TestCheckOutdatedIgnoresWhenGenFileConditionIsFalse(t *testing.T) {
	const filename = "test.txt"

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
			GenerateFile(
				Labels(filename),
				Bool("condition", condition),
				Str("content", "some content"),
			).String(),
		)
	}

	// Checking detection when the config has condition = false
	createConfig(filename, false)
	assertOutdatedFiles([]string{})

	// Checking detection when the condition is set to true
	createConfig(filename, true)
	assertOutdatedFiles([]string{filename})

	s.Generate()
	assertOutdatedFiles([]string{})

	// Setting it back to false is detected as change since it should be deleted
	createConfig(filename, false)
	assertOutdatedFiles([]string{"test.txt"})

	s.Generate()

	assertOutdatedFiles([]string{})
}

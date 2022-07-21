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
)

func TestCheckReturnsOutdatedStackFilenamesForGeneratedHCL(t *testing.T) {
	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.RootDir(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	// Checking detection when there is no config generated yet
	assertOutdatedFiles([]string{})
	stackEntry.CreateConfig(
		generateHCL(
			labels("test.tf"),
			content(
				terraform(
					str("required_version", "1.10"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"test.tf"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Now checking when we have code + it gets outdated.
	stackEntry.CreateConfig(
		generateHCL(
			labels("test.tf"),
			content(
				terraform(
					str("required_version", "1.11"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"test.tf"})

	s.Generate()

	// Changing generated filenames will trigger detection,
	// with new + old filenames.
	stackEntry.CreateConfig(
		generateHCL(
			labels("testnew.tf"),
			content(
				terraform(
					str("required_version", "1.11"),
				),
			),
		).String())

	assertOutdatedFiles([]string{"test.tf", "testnew.tf"})

	// Adding new filename to generation trigger detection
	stackEntry.CreateConfig(
		hcldoc(
			generateHCL(
				labels("testnew.tf"),
				content(
					terraform(
						str("required_version", "1.11"),
					),
				),
			),
			generateHCL(
				labels("another.tf"),
				content(
					backend(
						labels("type"),
					),
				),
			),
		).String())

	assertOutdatedFiles([]string{"another.tf", "test.tf", "testnew.tf"})

	s.Generate()

	assertOutdatedFiles([]string{})

	// Detects configurations that have been removed.
	stackEntry.DeleteConfig()

	assertOutdatedFiles([]string{"another.tf", "testnew.tf"})

	s.Generate()

	assertOutdatedFiles([]string{})
}

func TestCheckOutdatedDetectsEmptyGenerateHCLBlocks(t *testing.T) {
	s := sandbox.New(t)

	stackEntry := s.CreateStack("stacks/stack")
	stack := stackEntry.Load()

	assertOutdatedFiles := func(want []string) {
		t.Helper()

		got, err := generate.CheckStack(s.RootDir(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	stackEntry.CreateConfig(
		generateHCL(
			labels("test.tf"),
			content(),
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

		got, err := generate.CheckStack(s.RootDir(), stack)
		assert.NoError(t, err)
		assertEqualStringList(t, got, want)
	}

	createConfig := func(filename string, condition bool) {
		stackEntry.CreateConfig(
			generateHCL(
				labels(filename),
				boolean("condition", condition),
				content(
					block("whatever"),
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

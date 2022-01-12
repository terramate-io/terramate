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
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCheckReturnsOutdatedStacks(t *testing.T) {
	hcl := hclwrite.NewHCL
	stack := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("stack", builders...)
	}
	backend := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("backend", builders...)
	}
	terramate := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("terramate", builders...)
	}
	exportAsLocals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("export_as_locals", builders...)
	}
	labels := hclwrite.Labels
	expr := hclwrite.Expression

	s := sandbox.New(t)

	stack1 := s.CreateStack("stacks/stack-1")
	stack2 := s.CreateStack("stacks/stack-2")

	stack1Dir := "/" + stack1.RelPath()
	stack2Dir := "/" + stack2.RelPath()

	got, err := generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{})

	// Checking detection when there is no config generated yet
	// for both locals and backend config
	stack1.CreateConfig(
		hcl(
			stack(),
			exportAsLocals(
				expr("test", "terramate.path"),
			),
		).String())

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{
		{
			StackDir: stack1Dir,
			Filename: generate.LocalsFilename,
		},
	})

	stack2.CreateConfig(
		hcl(
			terramate(
				backend(labels("test")),
			),
			stack(),
		).String())

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{
		{
			StackDir: stack1Dir,
			Filename: generate.LocalsFilename,
		},
		{
			StackDir: stack2Dir,
			Filename: generate.BackendCfgFilename,
		},
	})

	s.Generate()

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{})

	// Now checking when we have code + it gets outdated
	// for both locals and backend.
	stack1.CreateConfig(
		hcl(
			stack(),
			exportAsLocals(
				expr("changed", "terramate.name"),
			),
		).String())

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{
		{
			StackDir: stack1Dir,
			Filename: generate.LocalsFilename,
		},
	})

	stack2.CreateConfig(
		hcl(
			terramate(
				backend(labels("changed")),
			),
			stack(),
		).String())

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{
		{
			StackDir: stack1Dir,
			Filename: generate.LocalsFilename,
		},
		{
			StackDir: stack2Dir,
			Filename: generate.BackendCfgFilename,
		},
	})

	s.Generate()

	got, err = generate.Check(s.RootDir())
	assert.NoError(t, err)
	assertOutdatedEquals(t, got, []generate.Outdated{})
}

func TestCheckSucceedsOnEmptyProject(t *testing.T) {
	s := sandbox.New(t)
	got, err := generate.Check(s.RootDir())
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got))
}

func TestCheckFailsWithInvalidConfig(t *testing.T) {
	hcl := hclwrite.NewHCL
	stack := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("stack", builders...)
	}
	backend := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("backend", builders...)
	}
	terramate := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("terramate", builders...)
	}
	exportAsLocals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("export_as_locals", builders...)
	}
	expr := hclwrite.Expression
	labels := hclwrite.Labels

	invalidConfigs := []string{
		hcl(
			terramate(
				backend(
					labels("test"),
					expr("undefined", "terramate.undefined"),
				),
			),
			stack(),
		).String(),
		hcl(
			exportAsLocals(
				expr("undefined", "terramate.undefined"),
			),
			stack(),
		).String(),
	}

	for _, invalidConfig := range invalidConfigs {
		s := sandbox.New(t)
		_, err := generate.Check(s.RootDir())
		assert.NoError(t, err)

		stackEntry := s.CreateStack("stack")
		stackEntry.CreateConfig(invalidConfig)

		_, err = generate.Check(s.RootDir())
		assert.Error(t, err, "should fail for configuration:\n%s", invalidConfig)
	}
}

func TestCheckFailsIfPathDoesntExist(t *testing.T) {
	_, err := generate.Check(test.NonExistingDir(t))
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsNotDir(t *testing.T) {
	dir := t.TempDir()
	filename := "test"

	test.WriteFile(t, dir, filename, "whatever")
	path := filepath.Join(dir, filename)

	_, err := generate.Check(path)
	assert.Error(t, err)
}

func TestCheckFailsIfPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	relpath := test.RelPath(t, test.Getwd(t), dir)

	_, err := generate.Check(relpath)
	assert.Error(t, err)
}

func assertOutdatedEquals(t *testing.T, got []generate.Outdated, want []generate.Outdated) {
	t.Helper()

	assert.EqualInts(t, len(want), len(got), "want %+v != got %+v", want, got)
	for i, wv := range want {
		gv := got[i]
		if gv != wv {
			t.Errorf("got[%d][%+v] != want[%d][%+v]", i, gv, i, wv)
		}
	}
}

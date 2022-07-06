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

		test.AssertStackImports(t, s.RootDir(), got, []string{stackImport1, stackImport2})
	}
}

func TestCreateStackDefaults(t *testing.T) {
	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	cli.run("create", "stack")

	got := s.LoadStack("stack")

	assert.EqualStrings(t, "stack", got.Name(), "checking stack name")
	assert.EqualStrings(t, "stack", got.Desc(), "checking stack description")

	test.AssertStackImports(t, s.RootDir(), got, []string{})
}

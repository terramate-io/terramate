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

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCreateStack(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())

	const (
		stackName        = "stack name"
		stackDescription = "stack description"
		stackImport1     = "/core/file1.tm.hcl"
		stackImport2     = "/core/file2.tm.hcl"
		stackAfter1      = "stack-after-1"
		stackAfter2      = "stack-after-2"
		stackBefore1     = "stack-before-1"
		stackBefore2     = "stack-before-2"
		genFilename      = "file.txt"
		genFileContent   = "testing is fun"
	)

	createFile := func(path string) {
		abspath := filepath.Join(s.RootDir(), path)
		test.WriteFile(t, filepath.Dir(abspath), filepath.Base(abspath), "")
	}

	createFile(stackImport1)
	createFile(stackImport2)

	s.RootEntry().CreateFile("generate.tm.hcl", `
		generate_file "%s" {
		  content = "%s"
		}
	`, genFilename, genFileContent)

	stackPaths := []string{
		"stack-1",
		"/stack-2",
		"/stacks/stack-a",
		"stacks/stack-b",
	}

	for _, stackPath := range stackPaths {
		stackID := newStackID(t)
		res := cli.run("create", stackPath,
			"--id", stackID,
			"--name", stackName,
			"--description", stackDescription,
			"--import", stackImport1,
			"--import", stackImport2,
			"--after", stackAfter1,
			"--after", stackAfter2,
			"--before", stackBefore1,
			"--before", stackBefore2,
		)

		t.Logf("run create stack %s", stackPath)
		t.Logf("stdout: %s", res.Stdout)
		t.Logf("stderr: %s", res.Stderr)

		want := fmt.Sprintf("Created stack %s\n", stackPath)
		if stackPath[0] != '/' {
			want = fmt.Sprintf("Created stack /%s\n", stackPath)
		}

		assertRunResult(t, res, runExpected{
			Stdout: want,
		})

		got := s.LoadStack(stackPath)

		gotID, _ := got.ID()
		assert.EqualStrings(t, stackID, gotID)
		assert.EqualStrings(t, stackName, got.Name(), "checking stack name")
		assert.EqualStrings(t, stackDescription, got.Desc(), "checking stack description")
		test.AssertDiff(t, got.After(), []string{stackAfter1, stackAfter2}, "created stack has invalid after")
		test.AssertDiff(t, got.Before(), []string{stackBefore1, stackBefore2}, "created stack has invalid before")

		test.AssertStackImports(t, s.RootDir(), got.HostPath(), []string{stackImport1, stackImport2})

		stackEntry := s.StackEntry(stackPath)
		gotGenCode := stackEntry.ReadFile(genFilename)

		assert.EqualStrings(t, genFileContent, gotGenCode, "checking stack generated code")
	}
}

func TestCreateStackDefaults(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	cli.run("create", "stack")

	got := s.LoadStack("stack")

	assert.EqualStrings(t, "stack", got.Name(), "checking stack name")
	assert.EqualStrings(t, "stack", got.Desc(), "checking stack description")

	if len(got.After()) > 0 {
		t.Fatalf("want no after, got: %v", got.After())
	}

	if len(got.Before()) > 0 {
		t.Fatalf("want no before, got: %v", got.Before())
	}

	// By default the CLI generates an id with an UUID
	gotID, _ := got.ID()
	_, err := uuid.Parse(gotID)
	assert.NoError(t, err, "validating default UUID")

	test.AssertStackImports(t, s.RootDir(), got.HostPath(), []string{})
}

func TestCreateStackIgnoreExistingOnDefaultStackCfgFound(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.run("create", "stack"), runExpected{
		IgnoreStdout: true,
	})
	assertRunResult(t, cli.run("create", "stack"), runExpected{
		Status:       1,
		IgnoreStderr: true,
	})
	assertRun(t, cli.run("create", "stack", "--ignore-existing"))
}

func TestCreateStackIgnoreExistingOnStackFound(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:stack/non_default_cfg.tm:stack{\n}",
	})
	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.run("create", "stack"), runExpected{
		Status:       1,
		IgnoreStderr: true,
	})
	assertRun(t, cli.run("create", "stack", "--ignore-existing"))
}

func TestCreateStackIgnoreExistingFatalOnOtherErrors(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	root := s.RootEntry()
	root.CreateDir("stack")
	// Here we fail stack creating with an access error
	root.Chmod("stack", 0444)
	cli := newCLI(t, s.RootDir())

	assertRunResult(t, cli.run("create", "stack", "--ignore-existing"), runExpected{
		Status:       1,
		IgnoreStderr: true,
	})
}

func newStackID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

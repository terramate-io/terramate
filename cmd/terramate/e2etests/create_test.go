// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
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

		s.ReloadConfig()
		absStackPath := filepath.Join(s.RootDir(), filepath.FromSlash(stackPath))
		got := s.LoadStack(project.PrjAbsPath(s.RootDir(), absStackPath))

		assert.EqualStrings(t, stackID, got.ID)
		assert.EqualStrings(t, stackName, got.Name, "checking stack name")
		assert.EqualStrings(t, stackDescription, got.Description, "checking stack description")
		test.AssertDiff(t, got.After, []string{stackAfter1, stackAfter2}, "created stack has invalid after")
		test.AssertDiff(t, got.Before, []string{stackBefore1, stackBefore2}, "created stack has invalid before")

		test.AssertStackImports(t, s.RootDir(), got.HostDir(s.Config()), []string{stackImport1, stackImport2})

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

	got := s.LoadStack(project.NewPath("/stack"))

	assert.EqualStrings(t, "stack", got.Name, "checking stack name")
	assert.EqualStrings(t, "stack", got.Description, "checking stack description")

	if len(got.After) > 0 {
		t.Fatalf("want no after, got: %v", got.After)
	}

	if len(got.Before) > 0 {
		t.Fatalf("want no before, got: %v", got.Before)
	}

	// By default the CLI generates an id with an UUID
	_, err := uuid.Parse(got.ID)
	assert.NoError(t, err, "validating default UUID")

	test.AssertStackImports(t, s.RootDir(), got.HostDir(s.Config()), []string{})
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

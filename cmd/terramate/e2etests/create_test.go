// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
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

func TestCreateFailsWithIncompatibleFlags(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, args ...string) {
		s := sandbox.New(t)
		root := s.RootEntry()
		root.CreateDir("stack")
		cli := newCLI(t, s.RootDir())

		createArgs := []string{"create"}
		createArgs = append(createArgs, args...)

		assertRunResult(t, cli.run(createArgs...), runExpected{
			Status:      1,
			StderrRegex: "incompatible",
		})
	}

	t.Run("--all-terraform and path", func(t *testing.T) {
		test(t, "--all-terraform", "./stack")
	})

	t.Run("--all-terraform and --id", func(t *testing.T) {
		test(t, "--all-terraform", "--id=test")
	})

	t.Run("--all-terraform and --name", func(t *testing.T) {
		test(t, "--all-terraform", "--name=some-stack")
	})

	t.Run("--all-terraform and --description", func(t *testing.T) {
		test(t, "--all-terraform", "--description=desc")
	})

	t.Run("--all-terraform and --after", func(t *testing.T) {
		test(t, "--all-terraform", "--after=/test")
	})

	t.Run("--all-terraform and --before", func(t *testing.T) {
		test(t, "--all-terraform", "--before=/test")
	})

	t.Run("--all-terraform and --import", func(t *testing.T) {
		test(t, "--all-terraform", "--import=/test")
	})

	t.Run("--all-terraform and --ignore-existing", func(t *testing.T) {
		test(t, "--all-terraform", "--ignore-existing")
	})
}

func TestCreateWithAllTerraformModuleAtRoot(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:main.tf:terraform {
			backend "remote" {
				attr = "value"
			}
		}`,
		`f:README.md:# My module`,
	})
	tm := newCLI(t, s.RootDir())
	assertRunResult(t,
		tm.run("create", "--all-terraform"),
		runExpected{
			Stdout: "Created stack /\n",
		},
	)
	_, err := os.Lstat(filepath.Join(s.RootDir(), stack.DefaultFilename))
	assert.NoError(t, err)
}

func TestCreateWithAllTerraformModuleDeepDownInTheTree(t *testing.T) {
	testCase := func(t *testing.T, generate bool) {
		s := sandbox.New(t)
		const backendContent = `terraform {
		backend "remote" {
			attr = "value"
		}
	}

	`

		const providerContent = `
		provider "aws" {
			attr = 1
		}
	`

		const mixedBackendProvider = backendContent + providerContent

		s.BuildTree([]string{
			`f:prod/stacks/k8s-stack/deployment.yml:# empty file`,
			`f:prod/stacks/A/anyfile.tf:` + backendContent,
			`f:prod/stacks/A/README.md:# empty`,
			`f:prod/stacks/B/main.tf:` + providerContent,
			`f:prod/stacks/A/other-stack/main.tf:` + mixedBackendProvider,
			`f:README.md:# My module`,
			`f:generate.tm:generate_hcl "_generated.tf" {
			content {
				test = 1
			}
		}`,
		})
		tm := newCLI(t, s.RootDir())
		args := []string{"create", "--all-terraform"}
		if !generate {
			args = append(args, "--no-generate")
		}
		assertRunResult(t,
			tm.run(args...),
			runExpected{
				Stdout: `Created stack /prod/stacks/A
Created stack /prod/stacks/A/other-stack
Created stack /prod/stacks/B
`,
			},
		)

		for _, path := range []string{
			"/prod/stacks/A",
			"/prod/stacks/B",
			"/prod/stacks/A/other-stack",
		} {
			stackPath := filepath.Join(s.RootDir(), path)
			_, err := os.Lstat(filepath.Join(stackPath, stack.DefaultFilename))
			assert.NoError(t, err)

			_, err = os.Lstat(filepath.Join(stackPath, "_generated.tf"))
			if generate {
				assert.NoError(t, err)
			} else {
				errtest.Assert(t, err, os.ErrNotExist)
			}
		}
	}

	t.Run("with generation", func(t *testing.T) {
		testCase(t, true)
	})

	t.Run("without generation", func(t *testing.T) {
		testCase(t, false)
	})
}

func TestCreateWithAllTerraformSkipActualStacks(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{
		`s:stack`,
		`f:stack/main.tf:terraform {
			backend "remote" {
				attr = "value"
			}
		}`,
		`f:README.md:# My module`,
	})
	tm := newCLI(t, s.RootDir())
	assertRun(t, tm.run("create", "--all-terraform"))
}

func TestCreateWithAllTerraformDetectModulesInsideStacks(t *testing.T) {
	s := sandbox.New(t)
	const backendContent = `terraform {
		backend "remote" {
			attr = "value"
		}
	}`
	s.BuildTree([]string{
		`s:stack`,
		`f:stack/main.tf:` + backendContent,
		`f:stack/hidden/module/inside/stack/main.tf:` + backendContent,
		`f:README.md:# My module`,
	})
	tm := newCLI(t, s.RootDir())
	assertRunResult(t,
		tm.run("create", "--all-terraform"),
		runExpected{
			Stdout: "Created stack /stack/hidden/module/inside/stack\n",
		},
	)
}

func newStackID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCreateFailsWithIncompatibleFlags(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, stderrRegex string, args ...string) {
		t.Helper()
		s := sandbox.NoGit(t, true)
		root := s.RootEntry()
		root.CreateDir("stack")
		cli := NewCLI(t, s.RootDir())

		createArgs := []string{"create"}
		createArgs = append(createArgs, args...)

		AssertRunResult(t, cli.Run(createArgs...), RunExpected{
			Status:      1,
			StderrRegex: stderrRegex,
		})
	}

	testIncompatibleFlags := func(t *testing.T, args ...string) {
		test(t, "Invalid args", args...)
	}

	t.Run("without required arguments", func(t *testing.T) {
		test(t, "Missing args")
	})

	scanFlags := []string{
		"--all-terraform",
		"--all-terragrunt",
		"--ensure-stack-ids",
	}

	mainFlags := append([]string{
		"./stack",
	}, scanFlags...)

	pairs := map[string]struct{}{}
	for _, flag := range mainFlags {
		for _, otherFlag := range mainFlags {
			if flag == otherFlag {
				continue
			}
			args := []string{flag, otherFlag}
			sort.Strings(args)
			pairs[strings.Join(args, " ")] = struct{}{}
		}
	}

	for pair := range pairs {
		args := strings.Split(pair, " ")
		t.Run(fmt.Sprintf("%s conflicts with %s", args[0], args[1]), func(t *testing.T) {
			testIncompatibleFlags(t, args[0], args[1])
		})
	}

	nonScanFlags := []string{
		"--id=test",
		"--name=some-stack",
		"--description=desc",
		"--after=/test",
		"--before=/test",
		"--import=/test",
		"--ignore-existing",
	}

	for _, scanFlag := range scanFlags {
		for _, nonScanFlag := range nonScanFlags {
			t.Run(fmt.Sprintf("%s conflicts to %s", scanFlag, nonScanFlag), func(t *testing.T) {
				testIncompatibleFlags(t, scanFlag, nonScanFlag)
			})
		}
	}
}

func TestCreateStack(t *testing.T) {
	t.Parallel()

	const (
		stackName        = "stack name"
		stackDescription = "stack description"
		stackImport1     = "/core/file1.tm.hcl"
		stackImport2     = "/core/file2.tm.hcl"
		stackAfter1      = "stack-after-1"
		stackAfter2      = "stack-after-2"
		stackBefore1     = "stack-before-1"
		stackBefore2     = "stack-before-2"
		stackTag1        = "a"
		stackTag2        = "b"
		genFilename      = "file.txt"
		genFileContent   = "testing is fun"
	)

	createFile := func(s sandbox.S, path string) {
		abspath := filepath.Join(s.RootDir(), path)
		test.WriteFile(t, filepath.Dir(abspath), filepath.Base(abspath), "")
	}

	testCreate := func(t *testing.T, flags ...string) {
		s := sandbox.NoGit(t, true)
		cli := NewCLI(t, s.RootDir())
		createFile(s, stackImport1)
		createFile(s, stackImport2)

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
			args := []string{"create", stackPath, "--id", stackID}
			args = append(args, flags...)
			res := cli.Run(args...)

			want := fmt.Sprintf("Created stack %s\n", stackPath)
			if stackPath[0] != '/' {
				want = fmt.Sprintf("Created stack /%s\n", stackPath)
			}

			AssertRunResult(t, res, RunExpected{
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
			test.AssertDiff(t, got.Tags, []string{stackTag1, stackTag2})

			test.AssertStackImports(t, s.RootDir(), got.HostDir(s.Config()), []string{stackImport1, stackImport2})

			stackEntry := s.StackEntry(stackPath)
			gotGenCode := stackEntry.ReadFile(genFilename)

			assert.EqualStrings(t, genFileContent, gotGenCode, "checking stack generated code")
		}
	}

	testCreate(t,
		"--name", stackName,
		"--description", stackDescription,
		"--import", stackImport1,
		"--import", stackImport2,
		"--after", stackAfter1,
		"--after", stackAfter2,
		"--before", stackBefore1,
		"--before", stackBefore2,
		"--tags", stackTag1,
		"--tags", stackTag2,
	)

	testCreate(t,
		"--name", stackName,
		"--description", stackDescription,
		"--import", strings.Join([]string{stackImport1, stackImport2}, ","),
		"--after", strings.Join([]string{stackAfter1, stackAfter2}, ","),
		"--before", strings.Join([]string{stackBefore1, stackBefore2}, ","),
		"--tags", strings.Join([]string{stackTag1, stackTag2}, ","),
	)
}

func TestCreateStackDefaults(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	cli := NewCLI(t, s.RootDir())
	cli.Run("create", "stack")

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

	s := sandbox.NoGit(t, true)
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "stack"), RunExpected{
		IgnoreStdout: true,
	})
	AssertRunResult(t, cli.Run("create", "stack"), RunExpected{
		Status:       1,
		IgnoreStderr: true,
	})
	AssertRun(t, cli.Run("create", "stack", "--ignore-existing"))
}

func TestCreateStackIgnoreExistingOnStackFound(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:stack/non_default_cfg.tm:stack{\n}",
	})
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.Run("create", "stack"), RunExpected{
		Status:       1,
		IgnoreStderr: true,
	})
	AssertRun(t, cli.Run("create", "stack", "--ignore-existing"))
}

func TestCreateStackIgnoreExistingFatalOnOtherErrors(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	root := s.RootEntry()
	root.CreateFile("stack", "")
	// Here we fail stack creating because a file with the same name exists
	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.Run("create", "stack", "--ignore-existing"), RunExpected{
		Status:       1,
		IgnoreStderr: true,
	})
}

func TestCreateEnsureStackID(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		wd     string
	}

	for _, tc := range []testcase{
		{
			name: "single stack at root with id",
			layout: []string{
				`s:.:id=test`,
			},
		},
		{
			name: "single stack at root without id",
			layout: []string{
				`s:.`,
			},
		},
		{
			name: "single stack at root without id but wd not at root",
			layout: []string{
				`d:some/deep/dir/for/test`,
				`s:.`,
			},
			wd: `/some/deep/dir/for/test`,
		},
		{
			name: "mix of multiple stacks with and without id",
			layout: []string{
				`s:s1`,
				`s:s1/a1:id=test`,
				`s:s2`,
				`s:s3/a3:id=test2`,
				`s:s3/a1`,
				`s:a/b/c/d/e/f/g/h/stack`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testEnsureStackID(t, tc.wd, tc.layout)
		})
	}
}

func testEnsureStackID(t *testing.T, wd string, layout []string) {
	s := sandbox.NoGit(t, true)
	s.BuildTree(layout)
	if wd == "" {
		wd = s.RootDir()
	} else {
		wd = filepath.Join(s.RootDir(), filepath.FromSlash(wd))
	}
	tm := NewCLI(t, wd)
	AssertRunResult(
		t,
		tm.Run("create", "--ensure-stack-ids"),
		RunExpected{
			Status:       0,
			IgnoreStdout: true,
		},
	)

	s.ReloadConfig()
	for _, stackElem := range s.LoadStacks() {
		if stackElem.ID == "" {
			t.Fatalf("stack.id not generated for stack %s", stackElem.Dir())
		}
	}
}

func newStackID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

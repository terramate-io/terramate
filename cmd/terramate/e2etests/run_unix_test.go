// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package e2etest

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunLookPathFromStackEnviron(t *testing.T) {
	t.Parallel()

	run := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("run", builders...)
	}
	env := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("env", builders...)
	}

	const stackName = "stack"

	s := sandbox.New(t)

	root := s.RootEntry()

	srcFile, err := os.Open(testHelperBin)
	assert.NoError(t, err)

	defer func() { assert.NoError(t, srcFile.Close()) }()

	srcStat, err := os.Stat(testHelperBin)
	assert.NoError(t, err)

	programName := "my-cmd"

	srcPerm := srcStat.Mode().Perm()

	tdir := filepath.Join(s.RootDir(), "bin")
	test.MkdirAll2(t, tdir, 0777)
	dstFilename := filepath.Join(tdir, programName)

	dstFile, err := os.Create(dstFilename)
	assert.NoError(t, err)

	_, err = io.Copy(dstFile, srcFile)
	assert.NoError(t, err)

	assert.NoError(t, dstFile.Close())

	test.AssertChmod(t, dstFilename, srcPerm)

	_ = s.CreateStack(stackName)

	root.CreateFile("env.tm",
		Terramate(
			Config(
				run(
					env(
						Expr("PATH", `"${terramate.root.path.fs.absolute}/bin:${env.PATH}"`),
					),
				),
			),
		).String(),
	)

	git := s.Git()
	git.Add(".")
	git.CommitAll("first commit")

	tm := newCLI(t, s.RootDir())

	assertRunResult(t, tm.run("run", "--", programName, "echo", "Hello from myscript"),
		runExpected{
			Stdout: "Hello from myscript\n",
		})
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/fmt/fs"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
)

func TestFormatTreeReturnsEmptyResultsForEmptyDir(t *testing.T) {
	tmpdir := test.TempDir(t)
	root, err := config.LoadRoot(tmpdir)
	assert.NoError(t, err)
	got, err := fs.FormatTree(root, project.NewPath("/"))
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

func TestFormatTreeFailsOnNonAccessibleSubdir(t *testing.T) {
	const subdir = "subdir"
	tmpdir := test.TempDir(t)
	test.Mkdir(t, tmpdir, subdir)

	test.AssertChmod(t, filepath.Join(tmpdir, subdir), 0)
	defer test.AssertChmod(t, filepath.Join(tmpdir, subdir), 0755)

	root, err := config.LoadRoot(tmpdir)
	assert.NoError(t, err)
	_, err = fs.FormatTree(root, project.NewPath("/"))
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonAccessibleFile(t *testing.T) {
	const filename = "filename.tm"

	tmpdir := test.TempDir(t)
	test.WriteFile(t, tmpdir, filename, `globals{
	a = 2
		b = 3
	}`)

	test.AssertChmod(t, filepath.Join(tmpdir, filename), 0)
	defer test.AssertChmod(t, filepath.Join(tmpdir, filename), 0755)

	root, err := config.LoadRoot(tmpdir)
	assert.NoError(t, err)
	_, err = fs.FormatTree(root, project.NewPath("/"))
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonExistentDir(t *testing.T) {
	tmpdir := test.TempDir(t)
	root, err := config.LoadRoot(tmpdir)
	assert.NoError(t, err)
	_, err = fs.FormatTree(root, project.NewPath("/non-existent"))
	assert.Error(t, err)
}

func TestFormatTreeIgnoresNonTerramateFiles(t *testing.T) {
	const (
		subdirName      = ".dotdir"
		unformattedCode = `
a = 1
 b = "la"
	c = 666
  d = []
`
	)

	tmpdir := test.TempDir(t)
	test.WriteFile(t, tmpdir, ".file.tm", unformattedCode)
	test.WriteFile(t, tmpdir, "file.tf", unformattedCode)
	test.WriteFile(t, tmpdir, "file.hcl", unformattedCode)

	test.Mkdir(t, tmpdir, subdirName)
	subdir := filepath.Join(tmpdir, subdirName)
	test.WriteFile(t, subdir, ".file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm.hcl", unformattedCode)

	root, err := config.LoadRoot(tmpdir)
	assert.NoError(t, err)
	got, err := fs.FormatTree(root, project.NewPath("/"))
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

func TestFormatTreeSupportsTmSkip(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, dirName string) {
		const unformattedCode = `
a = 1
 b = "la"
	c = 666
  d = []
`

		tmpdir := test.TempDir(t)
		if dirName != "." {
			test.MkdirAll(t, filepath.Join(tmpdir, dirName))
		}
		subdir := filepath.Join(tmpdir, dirName)
		test.WriteFile(t, subdir, terramate.SkipFilename, "")
		test.WriteFile(t, subdir, "file.tm", unformattedCode)
		test.WriteFile(t, subdir, "file.tm", unformattedCode)
		test.WriteFile(t, subdir, "file.tm.hcl", unformattedCode)

		root, err := config.LoadRoot(tmpdir)
		assert.NoError(t, err)
		got, err := fs.FormatTree(root, project.NewPath("/"))
		assert.NoError(t, err)
		assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
	}

	t.Run("./.tmskip", func(t *testing.T) { test(t, ".") })
	t.Run("somedir/.tmskip", func(t *testing.T) { test(t, "somedir") })
	t.Run("somedir/otherdir/.tmskip", func(t *testing.T) { test(t, "somedir/otherdir") })
}

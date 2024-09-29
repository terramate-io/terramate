// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package info_test

import (
	stdos "os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/os"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclutils"
)

func TestRangeFromHCLRange(t *testing.T) {
	t.Parallel()
	// We create a dir to simulate how the path will look
	// like in different OS's, like Windows.
	rootdir := test.TempDir(t)
	path := filepath.Join("dir", "sub", "assert.tm")
	start := Start(1, 1, 0)
	end := End(3, 2, 37)
	hclrange := Mkrange(rootdir.Join(path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath().String())
	assertRangePath(t, tmrange, path)

	assert.EqualInts(t, hclrange.Start.Line, tmrange.Start().Line())
	assert.EqualInts(t, hclrange.Start.Column, tmrange.Start().Column())
	assert.EqualInts(t, hclrange.Start.Byte, tmrange.Start().Byte())

	assert.EqualInts(t, hclrange.End.Line, tmrange.End().Line())
	assert.EqualInts(t, hclrange.End.Column, tmrange.End().Column())
	assert.EqualInts(t, hclrange.End.Byte, tmrange.End().Byte())
}

func TestRangeStrRepr(t *testing.T) {
	t.Parallel()
	rootdir := test.TempDir(t)
	tmrange := info.NewRange(rootdir, Mkrange(
		rootdir.Join("dir", "assert.tm"),
		Start(1, 1, 0),
		End(3, 2, 37),
	))
	assert.EqualStrings(t, "/dir/assert.tm:1,1-3,2", tmrange.String())

	tmrange = info.NewRange(rootdir, Mkrange(
		rootdir.Join("assert.tm"),
		Start(1, 1, 0),
		End(1, 2, 37),
	))
	assert.EqualStrings(t, "/assert.tm:1,1-2", tmrange.String())
}

func TestRangeWithFileOnRootdir(t *testing.T) {
	t.Parallel()
	rootdir := test.TempDir(t)
	path := "assert.tm"
	start := Start(0, 0, 0)
	end := End(0, 0, 0)
	hclrange := Mkrange(rootdir.Join(path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath().String())
	assertRangePath(t, tmrange, path)
}

func TestRangeOnRootWithFileOnRootdir(t *testing.T) {
	t.Parallel()
	rootdir := os.NewHostPath(string(stdos.PathSeparator))
	path := "assert.tm"
	start := Start(0, 0, 0)
	end := End(0, 0, 0)
	hclrange := Mkrange(rootdir.Join(path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath().String())
	assertRangePath(t, tmrange, path)
}

func assertRangePath(t *testing.T, tmrange info.Range, path string) {
	t.Helper()

	wantPath := project.NewPath("/" + filepath.ToSlash(path))
	if tmrange.Path() != wantPath {
		t.Fatalf("range.Path() = %q; want = %q", tmrange.Path(), wantPath)
	}
}

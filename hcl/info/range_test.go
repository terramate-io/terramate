// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package info_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclutils"
)

func TestRangeFromHCLRange(t *testing.T) {
	// We create a dir to simulate how the path will look
	// like in different OS's, like Windows.
	rootdir := t.TempDir()
	path := filepath.Join("dir", "sub", "assert.tm")
	start := Start(1, 1, 0)
	end := End(3, 2, 37)
	hclrange := Mkrange(filepath.Join(rootdir, path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath())
	assertRangePath(t, tmrange, path)

	assert.EqualInts(t, hclrange.Start.Line, tmrange.Start().Line())
	assert.EqualInts(t, hclrange.Start.Column, tmrange.Start().Column())
	assert.EqualInts(t, hclrange.Start.Byte, tmrange.Start().Byte())

	assert.EqualInts(t, hclrange.End.Line, tmrange.End().Line())
	assert.EqualInts(t, hclrange.End.Column, tmrange.End().Column())
	assert.EqualInts(t, hclrange.End.Byte, tmrange.End().Byte())
}

func TestRangeStrRepr(t *testing.T) {
	rootdir := t.TempDir()
	tmrange := info.NewRange(rootdir, Mkrange(
		filepath.Join(rootdir, "dir", "assert.tm"),
		Start(1, 1, 0),
		End(3, 2, 37),
	))
	assert.EqualStrings(t, "/dir/assert.tm:1,1-3,2", tmrange.String())

	tmrange = info.NewRange(rootdir, Mkrange(
		filepath.Join(rootdir, "assert.tm"),
		Start(1, 1, 0),
		End(1, 2, 37),
	))
	assert.EqualStrings(t, "/assert.tm:1,1-2", tmrange.String())
}

func TestRangeWithFileOnRootdir(t *testing.T) {
	rootdir := t.TempDir()
	path := "assert.tm"
	start := Start(0, 0, 0)
	end := End(0, 0, 0)
	hclrange := Mkrange(filepath.Join(rootdir, path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath())
	assertRangePath(t, tmrange, path)
}

func TestRangeOnRootWithFileOnRootdir(t *testing.T) {
	rootdir := string(os.PathSeparator)
	path := "assert.tm"
	start := Start(0, 0, 0)
	end := End(0, 0, 0)
	hclrange := Mkrange(filepath.Join(rootdir, path), start, end)
	tmrange := info.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.HostPath())
	assertRangePath(t, tmrange, path)
}

func assertRangePath(t *testing.T, tmrange info.Range, path string) {
	t.Helper()

	wantPath := project.NewPath("/" + filepath.ToSlash(path))
	if tmrange.Path() != wantPath {
		t.Fatalf("range.Path() = %q; want = %q", tmrange.Path(), wantPath)
	}
}

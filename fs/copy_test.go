// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCopyIfAllFilesAreFilteredDirIsNotCreated(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:test/1",
		"f:test/2",
		"f:test/3",
		"f:test/sub/notcopy",
		"f:test/sub/sub2/notcopy",
		"f:test/sub/sub2/sub3/notcopy",
		"f:test/anothersub/sub2/sub3/notcopy",
		"f:test3/notcopy",
	})

	destdir := test.TempDir(t)
	err := fs.CopyDir(destdir, s.RootDir(), func(path string, entry os.DirEntry) bool {
		return entry.Name() != "notcopy" &&
			entry.Name() != "root.config.tm"
	})

	assert.NoError(t, err)

	entries, err := os.ReadDir(destdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 1, len(entries))
	assert.EqualStrings(t, "test", entries[0].Name())

	entries, err = os.ReadDir(filepath.Join(destdir, "test"))
	assert.NoError(t, err)
	assert.EqualInts(t, 3, len(entries))

	for _, entry := range entries {
		assert.IsTrue(t, !entry.IsDir())
		switch entry.Name() {
		case "1", "2", "3":
			continue
		default:
			t.Fatalf("unexpected entry: %v", entry.Name())
		}
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

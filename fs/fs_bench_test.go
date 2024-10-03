// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs_test

import (
	"fmt"
	stdos "os"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/os"
)

func BenchmarkListFiles(b *testing.B) {
	const tmFiles = 50
	const otherFiles = 50
	const ndirs = 50
	b.StopTimer()
	dir := os.NewHostPath(b.TempDir())

	for i := 0; i < ndirs; i++ {
		p := dir.Join(fmt.Sprintf("dir_%d", i))
		err := stdos.MkdirAll(p.String(), 0644)
		assert.NoError(b, err)
	}

	for i := 0; i < tmFiles; i++ {
		p := dir.Join(fmt.Sprintf("terramate_%d.tm", i))
		f, err := stdos.Create(p.String())
		assert.NoError(b, err)
		assert.NoError(b, f.Close())
	}

	for i := 0; i < otherFiles; i++ {
		p := dir.Join(fmt.Sprintf("other_%d.txt", i))
		f, err := stdos.Create(p.String())
		assert.NoError(b, err)
		assert.NoError(b, f.Close())
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		files, err := fs.ListTerramateFiles(dir)
		if err != nil {
			b.Fatal(err)
		}
		if len(files) != tmFiles {
			b.Fatal("wrong number of tm files")
		}
	}
}

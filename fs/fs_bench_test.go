package fs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/fs"
)

func BenchmarkListFiles(b *testing.B) {
	const tmFiles = 50
	const otherFiles = 50
	b.StopTimer()
	dir := b.TempDir()
	for i := 0; i < tmFiles; i++ {
		p := filepath.Join(dir, fmt.Sprintf("terramate_%d.tm", i))
		f, err := os.Create(p)
		assert.NoError(b, err)
		assert.NoError(b, f.Close())
	}

	for i := 0; i < otherFiles; i++ {
		p := filepath.Join(dir, fmt.Sprintf("other_%d.txt", i))
		f, err := os.Create(p)
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

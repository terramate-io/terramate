// Copyright 2023 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	const ndirs = 50
	b.StopTimer()
	dir := b.TempDir()

	for i := 0; i < ndirs; i++ {
		p := filepath.Join(dir, fmt.Sprintf("dir_%d", i))
		err := os.MkdirAll(p, 0644)
		assert.NoError(b, err)
	}

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

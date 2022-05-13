// Copyright 2022 Mineiros GmbH
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

package hcl_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
)

// TODO(katcipis)
// hcl.FormatTreeInPlace
// hcl.FormatTreeDiff ? (for the diff)

func TestFormatHCL(t *testing.T) {
	type testcase struct {
		name  string
		input string
		want  string
	}

	tcases := []testcase{
		{
			name: "attributes alignment",
			input: `
a = 1
 b = "la"
	c = 666
  d = []
`,
			want: `
a = 1
b = "la"
c = 666
d = []
`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := hcl.Format(tcase.input)
			assert.EqualStrings(t, tcase.want, got)
		})

		t.Run("File/"+tcase.name, func(t *testing.T) {
			const filename = "file.tm"

			tmpdir := t.TempDir()
			test.WriteFile(t, tmpdir, filename, tcase.input)
			got, err := hcl.FormatFile(filepath.Join(tmpdir, filename))
			assert.NoError(t, err)
			assert.EqualStrings(t, tcase.want, got)
		})

		t.Run("Tree/"+tcase.name, func(t *testing.T) {
			const (
				filename   = "file.tm"
				subdirName = "subdir"
			)

			rootdir := t.TempDir()
			test.Mkdir(t, rootdir, subdirName)
			subdir := filepath.Join(rootdir, subdirName)

			test.WriteFile(t, rootdir, filename, tcase.input)
			test.WriteFile(t, subdir, filename, tcase.input)

			got, err := hcl.FormatTree(rootdir)
			assert.NoError(t, err)
			assert.EqualInts(t, 2, len(got), "want 2 formatted files, got: %v", got)

			for _, res := range got {
				assert.EqualStrings(t, tcase.want, res.Formatted)
			}

			wantFilepath := filepath.Join(rootdir, filename)
			wantSubdirFilepath := filepath.Join(subdir, filename)

			assert.EqualStrings(t, wantFilepath, got[0].Path)
			assert.EqualStrings(t, wantSubdirFilepath, got[1].Path)
		})
	}
}

func TestFormatFileDoesntExist(t *testing.T) {
	tmpdir := t.TempDir()
	_, err := hcl.FormatFile(filepath.Join(tmpdir, "dontexist.tm"))
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

	tmpdir := t.TempDir()
	test.WriteFile(t, tmpdir, ".file.tm", unformattedCode)
	test.WriteFile(t, tmpdir, "file.tf", unformattedCode)
	test.WriteFile(t, tmpdir, "file.hcl", unformattedCode)

	test.Mkdir(t, tmpdir, subdirName)
	subdir := filepath.Join(tmpdir, subdirName)
	test.WriteFile(t, subdir, ".file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm", unformattedCode)

	got, err := hcl.FormatTree(tmpdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

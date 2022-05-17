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
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"

	errtest "github.com/mineiros-io/terramate/test/errors"
)

// TODO(katcipis)
// hcl.FormatTreeDiff ? (for the diff)

func TestFormatHCL(t *testing.T) {
	type testcase struct {
		name     string
		input    string
		want     string
		wantErrs []error
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
		{
			name: "fails on syntax errors",
			input: `
				string = hi"
				bool   = rue
				list   = [
				obj    = {
			`,
			wantErrs: []error{
				errors.E(hcl.ErrHCLSyntax),
				errors.E(mkrange(start(2, 17, 17), end(3, 1, 18))),
				errors.E(mkrange(start(3, 17, 34), end(4, 1, 35))),
				errors.E(mkrange(start(4, 15, 49), end(5, 1, 50))),
				errors.E(mkrange(start(5, 15, 64), end(6, 1, 65))),
				errors.E(mkrange(start(2, 16, 16), end(2, 17, 17))),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			const filename = "test-input.hcl"
			got, err := hcl.Format(tcase.input, filename)

			addFilenameToErrorsFileRanges(tcase.wantErrs, filename)
			errtest.AssertErrorList(t, err, tcase.wantErrs)
			assert.EqualStrings(t, tcase.want, got)
		})

		t.Run("File/"+tcase.name, func(t *testing.T) {
			const filename = "file.tm"

			tmpdir := t.TempDir()
			test.WriteFile(t, tmpdir, filename, tcase.input)
			path := filepath.Join(tmpdir, filename)
			got, err := hcl.FormatFile(path)

			addFilenameToErrorsFileRanges(tcase.wantErrs, path)
			errtest.AssertErrorList(t, err, tcase.wantErrs)
			assert.EqualStrings(t, tcase.want, got, "checking formatted code")
			assertFileContains(t, filepath.Join(tmpdir, filename), tcase.input)
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

			// Since we have identical files we expect the same
			// set of errors for each filepath to be present.
			wantFilepath := filepath.Join(rootdir, filename)
			wantSubdirFilepath := filepath.Join(subdir, filename)
			wantErrs := []error{}

			for _, path := range []string{wantFilepath, wantSubdirFilepath} {
				for _, wantErr := range tcase.wantErrs {
					if e, ok := wantErr.(*errors.Error); ok {
						err := *e
						err.FileRange.Filename = path
						wantErrs = append(wantErrs, &err)
						continue
					}

					wantErrs = append(wantErrs, wantErr)
				}

			}
			errtest.AssertErrorList(t, err, wantErrs)
			if err != nil {
				return
			}
			assert.EqualInts(t, 2, len(got), "want 2 formatted files, got: %v", got)

			for _, res := range got {
				assert.EqualStrings(t, tcase.want, res.Formatted())
				assertFileContains(t, res.Path(), tcase.input)
			}

			assert.EqualStrings(t, wantFilepath, got[0].Path())
			assert.EqualStrings(t, wantSubdirFilepath, got[1].Path())

			t.Run("saving format results", func(t *testing.T) {
				for _, res := range got {
					assert.NoError(t, res.Save())
					assertFileContains(t, res.Path(), res.Formatted())
				}
			})
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

func assertFileContains(t *testing.T, filepath, got string) {
	t.Helper()

	data, err := os.ReadFile(filepath)
	assert.NoError(t, err, "reading file")

	want := string(data)
	assert.EqualStrings(t, want, got, "file %q contents don't match", filepath)
}

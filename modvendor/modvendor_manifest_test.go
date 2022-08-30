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

package modvendor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestVendorManifest(t *testing.T) {
	type (
		manifestConfig struct {
			path string
			cfg  fmt.Stringer
		}
		testcase struct {
			name      string
			files     []string
			manifest  manifestConfig
			wantFiles []string
			wantErr   error
		}
	)

	testcases := []testcase{
		{
			name: "no manifest",
			files: []string{
				"/dir/file",
				"/file",
			},
			wantFiles: []string{
				"/dir/file",
				"/file",
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {

			repoSandbox := sandbox.New(t)
			for _, file := range tcase.files {
				path := filepath.Join(repoSandbox.RootDir(), file)
				test.WriteFile(t, filepath.Dir(path), filepath.Base(path), "")
			}

			// Remove the default README.md created by the sandbox
			test.RemoveFile(t, repoSandbox.RootDir(), "README.md")

			repogit := repoSandbox.Git()
			repogit.CommitAll("setup vendored repo")

			gitURL := "file://" + repoSandbox.RootDir()

			rootdir := t.TempDir()
			source := newSource(t, gitURL, "main")

			const vendordir = "/vendor"
			got := modvendor.Vendor(rootdir, vendordir, source)

			assert.IsError(t, got.Error, tcase.wantErr)
			if tcase.wantErr != nil {
				return
			}

			clonedir := modvendor.AbsVendorDir(rootdir, vendordir, source)
			wantFiles := make([]string, len(tcase.wantFiles))
			for i, wantFile := range tcase.wantFiles {
				wantFiles[i] = filepath.Join(clonedir, wantFile)
			}
			gotFiles := listFiles(t, clonedir)
			test.AssertDiff(t, gotFiles, wantFiles)
		})
	}
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)

	files := []string{}
	dirs := []string{}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
	}

	for _, dir := range dirs {
		files = append(files, listFiles(t, dir)...)
	}

	sort.Strings(files)
	return files
}

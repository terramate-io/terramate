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
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestVendorManifest(t *testing.T) {
	type (
		manifestConfig struct {
			path     string
			patterns []string
		}
		testcase struct {
			name      string
			files     []string
			manifests []manifestConfig
			wantFiles []string
		}
	)

	testcases := []testcase{
		{
			name: "no manifest vendor all",
			files: []string{
				"/dir/file",
				"/file",
			},
			wantFiles: []string{
				"/dir/file",
				"/file",
			},
		},
		{
			name: "empty manifest vendor all",
			files: []string{
				"/dir/file",
				"/file",
			},
			manifests: []manifestConfig{
				{
					path:     "/manifest.tm",
					patterns: []string{},
				},
			},
			wantFiles: []string{
				"/dir/file",
				"/file",
				"/manifest.tm",
			},
		},
		{
			name: "filter single file",
			files: []string{
				"/dir/file",
				"/file",
			},
			manifests: []manifestConfig{
				{
					path: "/manifest.tm",
					patterns: []string{
						"/file",
					},
				},
			},
			wantFiles: []string{
				"/file",
			},
		},
		{
			name: "filter patterns and dirs and files",
			files: []string{
				"/main.tf",
				"/vars.tf",
				"/fun.tf",
				"/README.md",
				"/LICENSE",
				"/examples/1/main.tf",
				"/examples/2/main.tf",
				"/test/1/main.tf",
				"/test/2/main.tf",
				"/other/ohno.txt",
			},
			manifests: []manifestConfig{
				{
					path: "/manifest.tm",
					patterns: []string{
						"/*.tf",
						"/README.*",
						"/LICENSE",
						"examples",
					},
				},
			},
			wantFiles: []string{
				"/LICENSE",
				"/README.md",
				"/examples/1/main.tf",
				"/examples/2/main.tf",
				"/fun.tf",
				"/main.tf",
				"/vars.tf",
			},
		},
		{
			name: "filter config on .terramate",
			files: []string{
				"/main.tf",
				"/vars.tf",
				"/fun.tf",
				"/README.md",
				"/LICENSE",
				"/examples/1/main.tf",
				"/examples/2/main.tf",
				"/test/1/main.tf",
				"/test/2/main.tf",
				"/other/ohno.txt",
			},
			manifests: []manifestConfig{
				{
					path: ".terramate/manifest.tm",
					patterns: []string{
						"/*.tf",
						"/README.*",
						"/LICENSE",
						"examples",
					},
				},
			},
			wantFiles: []string{
				"/LICENSE",
				"/README.md",
				"/examples/1/main.tf",
				"/examples/2/main.tf",
				"/fun.tf",
				"/main.tf",
				"/vars.tf",
			},
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			repoSandbox := sandbox.New(t)

			// Remove the default README.md created by the sandbox
			test.RemoveFile(t, repoSandbox.RootDir(), "README.md")

			for _, file := range tcase.files {
				path := filepath.Join(repoSandbox.RootDir(), file)
				test.WriteFile(t, filepath.Dir(path), filepath.Base(path), "")
			}

			for _, manifest := range tcase.manifests {
				path := filepath.Join(repoSandbox.RootDir(), manifest.path)
				patternList := "["
				for _, pattern := range manifest.patterns {
					patternList += fmt.Sprintf("%q,\n", pattern)
				}
				patternList += "]"
				hcldoc := Vendor(
					Manifest(
						Default(
							EvalExpr(t, "files", patternList),
						),
					),
				)
				test.WriteFile(t, filepath.Dir(path), filepath.Base(path), hcldoc.String())
			}

			repogit := repoSandbox.Git()
			repogit.CommitAll("setup vendored repo")

			gitURL := "file://" + repoSandbox.RootDir()

			rootdir := t.TempDir()
			source := newSource(t, gitURL, "main")

			const vendordir = "/vendor"
			got := modvendor.Vendor(rootdir, vendordir, source)
			assert.NoError(t, got.Error)

			clonedir := modvendor.AbsVendorDir(rootdir, vendordir, source)
			gotFiles := listFiles(t, clonedir)
			for i, f := range gotFiles {
				gotFiles[i] = strings.TrimPrefix(f, clonedir)
			}
			test.AssertDiff(t, gotFiles, tcase.wantFiles)
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

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package download_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/modvendor"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/uri"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
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
			name: "no manifest wont vendor .terramate",
			files: []string{
				"/file",
				"/.terramate/ignored_by_default",
				"/.terramate/subdir/ignored_by_default",
			},
			wantFiles: []string{
				"/file",
			},
		},
		{
			name: "no manifest will vendor .terramate on subdirs",
			files: []string{
				"/file/.terramate",
			},
			wantFiles: []string{
				"/file/.terramate",
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
			name: "empty manifest wont vendor .terramate",
			files: []string{
				"/a/b/c",
				"/c/d/e",
				"/.terramate/subdir/ignored_by_default",
			},
			manifests: []manifestConfig{
				{
					path:     "/.terramate/manifest.tm",
					patterns: []string{},
				},
			},
			wantFiles: []string{
				"/a/b/c",
				"/c/d/e",
			},
		},
		{
			name: "empty manifest will vendor .terramate on subdirs",
			files: []string{
				"/a/b/c/.terramate",
			},
			manifests: []manifestConfig{
				{
					path:     "/.terramate/manifest.tm",
					patterns: []string{},
				},
			},
			wantFiles: []string{
				"/a/b/c/.terramate",
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
			name: "double-star match against directory",
			files: []string{
				"/b/test2/main.tf",
				"/a/test2/main.tf",
				"/a/b/c/test2/main.tf",
			},
			manifests: []manifestConfig{
				{
					path: "/manifest.tm",
					patterns: []string{
						"**/test2/main.tf",
					},
				},
			},
			wantFiles: []string{
				"/a/b/c/test2/main.tf",
				"/a/test2/main.tf",
				"/b/test2/main.tf",
			},
		},
		{
			name: "double-star match against multiple directory",
			files: []string{
				"/b/test/test2/main.tf",
				"/a/test/test3/main.tf",
				"/a/b/test/c/main.tf",
				"excluded.tf",
			},
			manifests: []manifestConfig{
				{
					path: "/manifest.tm",
					patterns: []string{
						"**/test/**/main.tf",
					},
				},
			},
			wantFiles: []string{
				"/a/b/test/c/main.tf",
				"/a/test/test3/main.tf",
				"/b/test/test2/main.tf",
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
		{
			name: ".terramate has precedence over root for manifest config",
			files: []string{
				"/file1",
				"/file2",
			},
			manifests: []manifestConfig{
				{
					path:     "manifest.tm",
					patterns: []string{"/file1"},
				},
				{
					path:     ".terramate/manifest.tm",
					patterns: []string{"/file2"},
				},
			},
			wantFiles: []string{
				"/file2",
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

			gitURI := uri.File(repoSandbox.RootDir())
			rootdir := t.TempDir()
			source := newSource(t, gitURI, "main")

			vendordir := project.NewPath("/vendor")
			got := download.Vendor(rootdir, vendordir, source, nil)
			assert.NoError(t, got.Error)

			clonedir := modvendor.AbsVendorDir(rootdir, vendordir, source)
			gotFiles := listFiles(t, clonedir)
			for i, f := range gotFiles {
				gotFiles[i] = filepath.ToSlash(strings.TrimPrefix(f, clonedir))
			}
			test.AssertDiff(t, gotFiles, tcase.wantFiles)
		})
	}
}

func TestInvalidManifestFailsOnRoot(t *testing.T) {
	testInvalidManifestFails(t, "manifest.tm")
}

func TestInvalidManifestFailsDotTerramate(t *testing.T) {
	testInvalidManifestFails(t, ".terramate/manifest.tm")
}

func testInvalidManifestFails(t *testing.T, configpath string) {
	repoSandbox := sandbox.New(t)
	test.WriteFile(t, repoSandbox.RootDir(), configpath, "not valid HCL")

	repogit := repoSandbox.Git()
	repogit.CommitAll("setup vendored repo")

	gitURL := uri.File(repoSandbox.RootDir())
	source := newSource(t, gitURL, "main")

	got := download.Vendor(t.TempDir(), project.NewPath("/vendor"), source, nil)

	assert.EqualInts(t, 0, len(got.Vendored), "vendored should be empty")
	assert.EqualInts(t, 1, len(got.Ignored), "should have single ignored")
	assert.IsError(t, got.Ignored[0].Reason, errors.E(hcl.ErrHCLSyntax))
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

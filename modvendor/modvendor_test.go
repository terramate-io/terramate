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
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

// vendorPathSpec describes paths inside a vendor directory when its
// directory is not yet know. It's format is just:
//   <module source>#<vendor relative path>
// Where the module source can use the "text/template" (dot) variable to be
// substituted at a later time when the final vendor directory is computed.
//
// Example:
//   git::file://{{.}}/module-test?ref=main#main.tf
type vendorPathSpec string

type testcase struct {
	name         string
	source       string
	layout       []string
	vendordir    string
	configs      []hclconfig
	wantVendored []string
	wantIgnored  []wantIgnoredVendor
	wantError    error

	// wantFiles describes file contents that must exist in the vendor directory.
	// The fmt.Stringer can be a template that gets compiled with text/template
	// package and having access to the relative paths of the vendored modules.
	// For example, if `wantVendored` contains [<modsrc1>, <modsrc2>, ...], then
	// the template will have access to the list [<relative path of modsrc1>,
	// <relative path of modsrc2>, ...].
	// Then the template can reference the third element of the list:
	//   source = {{index 2}}
	// If this looks like a hack to you, it's because it is.
	wantFiles map[vendorPathSpec]fmt.Stringer
}

type hclconfig struct {
	repo string
	path string
	data fmt.Stringer
}

type wantIgnoredVendor struct {
	RawSource     string
	ReasonPattern string
}

type wantReport struct {
	Vendored []string
	Ignored  []wantIgnoredVendor
	Error    error
}

func TestModVendorAllRecursive(t *testing.T) {
	tcases := []testcase{
		{
			name: "module with no remote deps",
			layout: []string{
				"g:module-test",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "./modules/non-existent"),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
			},
		},
		{
			name: "module with HCL errors",
			layout: []string{
				"g:module-test",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: bytes.NewBufferString("this is not a valid HCL file"),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
			},
			wantError: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name: "module with ignored remote deps",
			layout: []string{
				"g:module-test",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "https://example.com/my-module"),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
			},
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource:     "https://example.com/my-module",
					ReasonPattern: "unsupported module source",
				},
			},
		},
		{
			name:   "module not found",
			source: "git::file://{{.}}/module-that-does-not-exists?ref=main",
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource:     "git::file://{{.}}/module-that-does-not-exists?ref=main",
					ReasonPattern: "failed to vendor",
				},
			},
		},
		{
			name: "module with 1 remote dependency",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::file://{{.}}/another-module?ref=main"),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/module-test?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
				"git::file://{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency that contains bogus module.source",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::file://{{.}}/another-module?ref=main"),
					),
				},
				{
					repo: "another-module",
					path: "another-module/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::"),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/module-test?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
				"git::file://{{.}}/another-module?ref=main",
			},
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource:     "git::",
					ReasonPattern: "reference must be non-empty",
				},
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times in same file",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/module-test?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
				),
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
				"git::file://{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times using an alternate vendordir",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			vendordir: "/strange/path",
			source:    "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/module-test?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
					),
				),
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
				"git::file://{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times in different files",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::file://{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/1.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
					),
				},
				{
					repo: "module-test",
					path: "module-test/2.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::file://{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/module-test?ref=main",
				"git::file://{{.}}/another-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/module-test?ref=main#1.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::file://{{.}}/module-test?ref=main#2.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 1}}"),
						Str("other", "value"),
					),
					Module(
						Labels("test2"),
						Str("source", "{{index . 1}}"),
					),
				),
			},
		},
		{
			name: "my-module -> (awesome-module -> cool-module, best-module)",
			layout: []string{
				"g:my-module",
				"g:awesome-module",
				"g:best-module",
				"g:cool-module",
			},
			source: "git::file://{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/cool-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/my-module?ref=main",
				"git::file://{{.}}/awesome-module?ref=main",
				"git::file://{{.}}/cool-module?ref=main",
				"git::file://{{.}}/best-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::file://{{.}}/my-module?ref=main#other.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 3}}"),
						Str("other", "value"),
					),
					Module(
						Labels("test2"),
						Str("source", "{{index . 1}}"),
					),
				),
				"git::file://{{.}}/awesome-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 2}}"),
					),
				),
			},
		},
		{
			name: "my-module -> (awesome-module -> cool-module -> forgotten-module, best-module)",
			layout: []string{
				"g:my-module",
				"g:awesome-module",
				"g:best-module",
				"g:cool-module",
				"g:forgotten-module",
			},
			source: "git::file://{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/cool-module?ref=main"),
						),
					),
				},
				{
					repo: "cool-module",
					path: "cool-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/forgotten-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/my-module?ref=main",
				"git::file://{{.}}/awesome-module?ref=main",
				"git::file://{{.}}/cool-module?ref=main",
				"git::file://{{.}}/best-module?ref=main",
				"git::file://{{.}}/forgotten-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::file://{{.}}/my-module?ref=main#other.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 3}}"),
						Str("other", "value"),
					),
					Module(
						Labels("test2"),
						Str("source", "{{index . 1}}"),
					),
				),
				"git::file://{{.}}/awesome-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 2}}"),
					),
				),
				"git::file://{{.}}/cool-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 4}}"),
					),
				),
			},
		},
		{
			name: "cycle: my-module -> (awesome-module -> cool-module -> my-module, best-module)",
			layout: []string{
				"g:my-module",
				"g:awesome-module",
				"g:best-module",
				"g:cool-module",
			},
			source: "git::file://{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::file://{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/cool-module?ref=main"),
						),
					),
				},
				{
					repo: "cool-module",
					path: "cool-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::file://{{.}}/my-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::file://{{.}}/my-module?ref=main",
				"git::file://{{.}}/awesome-module?ref=main",
				"git::file://{{.}}/cool-module?ref=main",
				"git::file://{{.}}/best-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::file://{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::file://{{.}}/my-module?ref=main#other.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 3}}"),
						Str("other", "value"),
					),
					Module(
						Labels("test2"),
						Str("source", "{{index . 1}}"),
					),
				),
				"git::file://{{.}}/awesome-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 2}}"),
					),
				),
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			if tc.vendordir == "" {
				tc.vendordir = "/vendor"
			}

			modulesDir := s.RootDir()
			rootdir := t.TempDir()
			for _, cfg := range tc.configs {
				test.WriteFile(t, s.RootDir(), cfg.path,
					fixupString(t, cfg.data.String(), modulesDir))

				git := sandbox.NewGit(t, filepath.Join(modulesDir, cfg.repo))
				git.CommitAll("files updated")
			}
			source := fixupString(t, tc.source, modulesDir)
			modsrc, err := tf.ParseSource(source)
			assert.NoError(t, err)
			got := modvendor.Vendor(rootdir, tc.vendordir, modsrc)
			want := fixupReport(t, wantReport{
				Vendored: tc.wantVendored,
				Ignored:  tc.wantIgnored,
				Error:    tc.wantError,
			}, modulesDir, tc.vendordir)

			assertVendorReport(t, want, got)
			checkWantedFiles(t, tc, modulesDir, rootdir, tc.vendordir)
		})
	}
}

func fixupString(t *testing.T, input string, value interface{}) string {
	srctpl, err := template.New("template").Parse(input)
	assert.NoError(t, err)
	var buf bytes.Buffer
	err = srctpl.Execute(&buf, value)
	assert.NoError(t, err)
	return buf.String()
}

func fixupReport(t *testing.T, r wantReport, value string, vendordir string) modvendor.Report {
	out := modvendor.Report{
		Vendored: make(map[string]modvendor.Vendored),
		Error:    r.Error,
	}
	for _, vendored := range r.Vendored {
		rawSource := fixupString(t, vendored, value)
		modsrc, err := tf.ParseSource(rawSource)
		assert.NoError(t, err)
		out.Vendored[rawSource] = modvendor.Vendored{
			Source: modsrc,
			Dir:    modvendor.Dir(vendordir, modsrc),
		}
	}
	for _, ignored := range r.Ignored {
		rawSource := fixupString(t, ignored.RawSource, value)
		reason := fixupString(t, ignored.ReasonPattern, value)
		out.Ignored = append(out.Ignored, modvendor.IgnoredVendor{
			RawSource: rawSource,
			Reason:    reason,
		})
	}
	return out
}

func evaluateWantedFiles(
	t *testing.T,
	wantFiles map[vendorPathSpec]fmt.Stringer,
	modulesDir string,
	rootdir string,
	vendordir string,
) map[string]fmt.Stringer {
	evaluated := map[string]fmt.Stringer{}
	for pathSpec, expectedStringer := range wantFiles {
		pathSpecParts := strings.Split(string(pathSpec), "#")
		assert.EqualInts(t, len(pathSpecParts), 2)
		source := pathSpecParts[0]
		path := pathSpecParts[1]

		modsrc, err := tf.ParseSource(fixupString(t, source, modulesDir))
		assert.NoError(t, err)
		absVendorDir := modvendor.AbsVendorDir(rootdir, vendordir, modsrc)
		evaluatedPath := filepath.Join(absVendorDir, path)
		evaluated[evaluatedPath] = expectedStringer
	}
	return evaluated
}

func checkWantedFiles(
	t *testing.T,
	tc testcase,
	modulesDir string,
	rootdir string,
	vendordir string,
) {
	wantFiles := evaluateWantedFiles(t, tc.wantFiles, modulesDir, rootdir, vendordir)
	vendorDir := filepath.Join(rootdir, tc.vendordir)

	if _, err := os.Stat(vendorDir); err != nil {
		if os.IsNotExist(err) {
			return
		}
		assert.Error(t, err)
	}

	err := filepath.Walk(vendorDir, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".tf" {
			return nil
		}

		expectedStringTemplate, ok := wantFiles[path]
		if ok {
			// file must be rewritten
			relVendoredPaths := computeRelativePaths(
				t, filepath.Dir(path), tc.wantVendored, modulesDir, rootdir, tc.vendordir,
			)
			want := fixupString(t, expectedStringTemplate.String(), relVendoredPaths)
			got := string(test.ReadFile(t, filepath.Dir(path), filepath.Base(path)))
			assert.EqualStrings(t, want, got, "file %q mismatch", path)
		} else {
			// check the vendored file is the same as the one in the module dir.
			originalPath := strings.TrimPrefix(path, vendorDir)
			pathEnd := strings.TrimPrefix(originalPath, modulesDir)
			originalPath = strings.TrimSuffix(originalPath, pathEnd)
			pathParts := strings.Split(pathEnd, "/")
			moduleName := pathParts[1]

			originalPath = filepath.Join(originalPath, moduleName, strings.Join(pathParts[3:], "/"))

			originalBytes, err := ioutil.ReadFile(originalPath)
			assert.NoError(t, err)

			gotBytes, err := ioutil.ReadFile(path)
			assert.NoError(t, err)
			assert.EqualStrings(t, string(originalBytes), string(gotBytes),
				"files %q and %q mismatch", originalPath, path)
		}
		return nil
	})
	assert.NoError(t, err)
}

func computeRelativePaths(
	t *testing.T,
	relativeToDir string,
	wantVendored []string,
	modulesDir string,
	rootdir string,
	vendordir string,
) []string {
	// TODO(i4k): assumes files are always at the root of the module.
	relVendoredPaths := []string{}
	for _, vendored := range wantVendored {
		rawSource := fixupString(t, vendored, modulesDir)
		modsrc, err := tf.ParseSource(rawSource)
		assert.NoError(t, err)
		relPath, err := filepath.Rel(relativeToDir,
			modvendor.AbsVendorDir(rootdir, vendordir, modsrc))
		assert.NoError(t, err)
		relVendoredPaths = append(relVendoredPaths, relPath)
	}
	return relVendoredPaths
}

func TestModVendorWithCommitIDRef(t *testing.T) {
	const (
		path     = "github.com/mineiros-io/example"
		filename = "test.txt"
		content  = "test"
	)

	repoSandbox := sandbox.New(t)

	repogit := repoSandbox.Git()

	repogit.CheckoutNew("branch")
	repoSandbox.RootEntry().CreateFile(filename, content)
	repogit.CommitAll("add file")

	ref := repogit.RevParse("branch")
	// So the initial clone gets the repo pointing at main as "default"
	repogit.Checkout("main")

	gitURL := "file://" + repoSandbox.RootDir()
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=%s", gitURL, ref))
	assert.NoError(t, err)

	const vendordir = "/dir/reftest/vendor"
	got := modvendor.Vendor(rootdir, vendordir, source)
	assertVendorReport(t, modvendor.Report{
		Vendored: map[string]modvendor.Vendored{
			source.Raw: {
				Source: source,
				Dir:    modvendor.Dir(vendordir, source),
			},
		},
	}, got)

	cloneDir := modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[source.Raw].Source)
	gotContent := test.ReadFile(t, cloneDir, filename)
	assert.EqualStrings(t, content, string(gotContent))
	assertNoGitDir(t, cloneDir)
}

func TestModVendorWithRef(t *testing.T) {
	const (
		path     = "github.com/mineiros-io/example"
		ref      = "main"
		filename = "test.txt"
		content  = "test"
	)

	repoSandbox := sandbox.New(t)

	repoSandbox.RootEntry().CreateFile(filename, content)

	repogit := repoSandbox.Git()
	repogit.CommitAll("add file")

	gitURL := "file://" + repoSandbox.RootDir()
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=%s", gitURL, ref))
	assert.NoError(t, err)

	const vendordir = "/vendor"
	got := modvendor.Vendor(rootdir, vendordir, source)
	assertVendorReport(t, modvendor.Report{
		Vendored: map[string]modvendor.Vendored{
			source.Raw: {
				Source: source,
				Dir:    modvendor.Dir(vendordir, source),
			},
		},
	}, got)

	cloneDir := got.Vendored[source.Raw].Dir
	wantCloneDir := modvendor.Dir(vendordir, source)
	assert.EqualStrings(t, wantCloneDir, cloneDir)

	absCloneDir := modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[source.Raw].Source)
	gotContent := test.ReadFile(t, absCloneDir, filename)
	assert.EqualStrings(t, content, string(gotContent))
	assertNoGitDir(t, absCloneDir)

	const (
		newRef      = "branch"
		newFilename = "new.txt"
		newContent  = "new"
	)

	repogit.CheckoutNew(newRef)
	repoSandbox.RootEntry().CreateFile(newFilename, newContent)
	repogit.CommitAll("add new file")
	// We need to checkout back to the initial branch
	// or else the test passes even if the correct ref is not used.
	repogit.Checkout(ref)

	source = tf.Source{
		URL:  gitURL,
		Ref:  newRef,
		Path: path,
	}
	got = modvendor.Vendor(rootdir, vendordir, source)

	wantCloneDir = modvendor.Dir(vendordir, source)
	newCloneDir := got.Vendored[source.Raw].Dir
	assert.EqualStrings(t, wantCloneDir, newCloneDir)

	absCloneDir = modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[source.Raw].Source)
	assertNoGitDir(t, absCloneDir)

	gotContent = test.ReadFile(t, absCloneDir, filename)
	assert.EqualStrings(t, content, string(gotContent))

	gotContent = test.ReadFile(t, absCloneDir, newFilename)
	assert.EqualStrings(t, newContent, string(gotContent))
}

func TestModVendorDoesNothingIfRefExists(t *testing.T) {
	s := sandbox.New(t)

	s.RootEntry().CreateFile("file.txt", "data")

	g := s.Git()
	g.CommitAll("add file")

	gitURL := "file://" + s.RootDir()
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=main", gitURL))
	assert.NoError(t, err)

	const vendordir = "/vendor/fun"
	clonedir := modvendor.AbsVendorDir(rootdir, vendordir, source)
	test.MkdirAll(t, clonedir)
	got := modvendor.Vendor(rootdir, vendordir, source)
	want := modvendor.Report{
		Ignored: []modvendor.IgnoredVendor{
			{
				RawSource: source.Raw,
				Reason:    string(modvendor.ErrAlreadyVendored),
			},
		},
	}
	assertVendorReport(t, want, got)

	entries := test.ReadDir(t, clonedir)
	if len(entries) > 0 {
		t.Fatalf("wanted clone dir to be empty, got: %v", entries)
	}
}

func TestModVendorNoRefFails(t *testing.T) {
	s := sandbox.New(t)
	gitURL := "file://" + s.RootDir()
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s", gitURL))
	assert.NoError(t, err)
	report := modvendor.Vendor(rootdir, "/vendor", source)

	assertVendorReport(t, modvendor.Report{
		Ignored: []modvendor.IgnoredVendor{
			{
				RawSource: source.Raw,
				Reason:    "reference must be non-empty",
			},
		},
	}, report)
}

func TestModVendorVendorDirIsRelativeFails(t *testing.T) {
	const (
		path = "github.com/mineiros-io/example"
	)

	s := sandbox.New(t)
	gitURL := "file://" + s.RootDir()
	rootdir := t.TempDir()

	report := modvendor.Vendor(rootdir, "../test", tf.Source{
		URL:  gitURL,
		Path: path,
		Ref:  "main",
	})

	assert.Error(t, report.Error)
}

func assertNoGitDir(t *testing.T, dir string) {
	t.Helper()

	entries := test.ReadDir(t, dir)
	for _, entry := range entries {
		if entry.Name() == ".git" {
			t.Fatalf("found unwanted .git inside %q", dir)
		}
	}
}

func assertVendorReport(t *testing.T, want, got modvendor.Report) {
	assert.EqualInts(t, len(want.Vendored), len(got.Vendored),
		"number of vendored is different: want %s != got %s",
		want.Verbose(), got.Verbose())
	assert.EqualInts(t, len(want.Ignored), len(got.Ignored),
		"number of ignored is different: want %s != got %s",
		want.Verbose(), got.Verbose())
	for i, wantVendor := range want.Vendored {
		if wantVendor != got.Vendored[i] {
			t.Errorf("want %v is different than %v",
				want.Verbose(), got.Verbose())
		}
	}
	for i, wantIgnored := range want.Ignored {
		if wantIgnored.RawSource != got.Ignored[i].RawSource {
			t.Errorf("want.RawSource %v is different than %v",
				wantIgnored.RawSource, got.Ignored[i].RawSource)
		}
		if wantIgnored.Reason != "" {
			if ok, _ := regexp.MatchString(wantIgnored.Reason, got.Ignored[i].Reason); !ok {
				t.Errorf("want.Reason %v is different than %v",
					wantIgnored.Reason, got.Ignored[i].Reason)
			}
		} else if got.Ignored[i].Reason != "" {
			t.Fatalf("unexpected reason %q", got.Ignored[i].Reason)
		}
	}

	errtest.Assert(t, got.Error, want.Error)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

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

package download_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/event"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/modvendor/download"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog"
	"go.lsp.dev/uri"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

// vendorPathSpec describes paths inside a vendor directory when its
// directory is not yet know. It's format is just:
//
//	<module source>#<vendor relative path>
//
// Where the module source can use the "text/template" (dot) variable to be
// substituted at a later time when the final vendor directory is computed.
//
// Example:
//
//	git::file://{{.}}/module-test?ref=main#main.tf
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
	RawSource string
	Error     error
}

type wantReport struct {
	Vendored []string
	Ignored  []wantIgnoredVendor
	Error    error
}

func TestDownloadVendor(t *testing.T) {
	t.Parallel()

	tcases := []testcase{
		{
			name: "module with no remote deps",
			layout: []string{
				"g:module-test",
			},
			source: "git::{{.}}/module-test?ref=main",
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
				"git::{{.}}/module-test?ref=main",
			},
		},
		{
			name: "module with HCL errors",
			layout: []string{
				"g:module-test",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: bytes.NewBufferString("this is not a valid HCL file"),
				},
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
			},
			wantError: errors.E(hcl.ErrHCLSyntax),
		},
		{
			name: "module with ignored remote deps",
			layout: []string{
				"g:module-test",
			},
			source: "git::{{.}}/module-test?ref=main",
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
				"git::{{.}}/module-test?ref=main",
			},
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource: "https://example.com/my-module",
					Error:     errors.E(download.ErrUnsupportedModSrc),
				},
			},
		},
		{
			name:   "module not found",
			source: "git::{{.}}/module-that-does-not-exists?ref=main",
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource: "git::{{.}}/module-that-does-not-exists?ref=main",
					Error:     errors.E(download.ErrDownloadMod),
				},
			},
		},
		{
			name: "module with 1 remote dependency",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::{{.}}/another-module?ref=main"),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency and subdir",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::{{.}}/another-module//sub/dir?ref=main"),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}/sub/dir"),
				),
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module//sub/dir?ref=main",
			},
		},
		{
			name: "module with N remote dependency and subdir",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Doc(
						Module(
							Labels("nosubdir"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("subdir1"),
							Str("source", "git::{{.}}/another-module//sub/dir?ref=main"),
						),
						Module(
							Labels("subdir2"),
							Str("source", "git::{{.}}/another-module//sub?ref=main"),
						),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#main.tf": Doc(
					Module(
						Labels("nosubdir"),
						Str("source", "{{index . 1}}"),
					),
					Module(
						Labels("subdir1"),
						Str("source", "{{index . 1}}/sub/dir"),
					),
					Module(
						Labels("subdir2"),
						Str("source", "{{index . 1}}/sub"),
					),
				),
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency that contains bogus module.source",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", "git::{{.}}/another-module?ref=main"),
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
				"git::{{.}}/module-test?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
			wantIgnored: []wantIgnoredVendor{
				{
					RawSource: "git::",
					Error:     errors.E(tf.ErrInvalidModSrc),
				},
			},
		},
		{
			name: "module with empty module.source",
			layout: []string{
				"g:module-test",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Module(
						Labels("test"),
						Str("source", ""),
					),
				},
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times in same file",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#main.tf": Doc(
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
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times using an alternate vendordir",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			vendordir: "/strange/path",
			source:    "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#main.tf": Doc(
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
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
		},
		{
			name: "module with 1 remote dependency referenced multiple times in different files",
			layout: []string{
				"g:module-test",
				"g:another-module",
			},
			source: "git::{{.}}/module-test?ref=main",
			configs: []hclconfig{
				{
					repo: "module-test",
					path: "module-test/1.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
					),
				},
				{
					repo: "module-test",
					path: "module-test/2.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/another-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::{{.}}/another-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::{{.}}/module-test?ref=main",
				"git::{{.}}/another-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/module-test?ref=main#1.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::{{.}}/module-test?ref=main#2.tf": Doc(
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
			source: "git::{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/cool-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::{{.}}/my-module?ref=main",
				"git::{{.}}/awesome-module?ref=main",
				"git::{{.}}/cool-module?ref=main",
				"git::{{.}}/best-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::{{.}}/my-module?ref=main#other.tf": Doc(
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
				"git::{{.}}/awesome-module?ref=main#main.tf": Doc(
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
			source: "git::{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/cool-module?ref=main"),
						),
					),
				},
				{
					repo: "cool-module",
					path: "cool-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/forgotten-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::{{.}}/my-module?ref=main",
				"git::{{.}}/awesome-module?ref=main",
				"git::{{.}}/cool-module?ref=main",
				"git::{{.}}/best-module?ref=main",
				"git::{{.}}/forgotten-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::{{.}}/my-module?ref=main#other.tf": Doc(
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
				"git::{{.}}/awesome-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 2}}"),
					),
				),
				"git::{{.}}/cool-module?ref=main#main.tf": Doc(
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
			source: "git::{{.}}/my-module?ref=main",
			configs: []hclconfig{
				{
					repo: "my-module",
					path: "my-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "my-module",
					path: "my-module/other.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/best-module?ref=main"),
							Str("other", "value"),
						),
						Module(
							Labels("test2"),
							Str("source", "git::{{.}}/awesome-module?ref=main"),
						),
					),
				},
				{
					repo: "awesome-module",
					path: "awesome-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/cool-module?ref=main"),
						),
					),
				},
				{
					repo: "cool-module",
					path: "cool-module/main.tf",
					data: Doc(
						Module(
							Labels("test"),
							Str("source", "git::{{.}}/my-module?ref=main"),
						),
					),
				},
			},
			wantVendored: []string{
				"git::{{.}}/my-module?ref=main",
				"git::{{.}}/awesome-module?ref=main",
				"git::{{.}}/cool-module?ref=main",
				"git::{{.}}/best-module?ref=main",
			},
			wantFiles: map[vendorPathSpec]fmt.Stringer{
				"git::{{.}}/my-module?ref=main#main.tf": Module(
					Labels("test"),
					Str("source", "{{index . 1}}"),
				),
				"git::{{.}}/my-module?ref=main#other.tf": Doc(
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
				"git::{{.}}/awesome-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 2}}"),
					),
				),
				"git::{{.}}/cool-module?ref=main#main.tf": Doc(
					Module(
						Labels("test"),
						Str("source", "{{index . 0}}"),
					),
				),
			},
		},
	}

	type fixture struct {
		rootdir       string
		vendorDir     project.Path
		modsrc        tf.Source
		uriModulesDir uri.URI
	}

	setup := func(t *testing.T, tc testcase) fixture {
		s := sandbox.New(t)
		s.BuildTree(tc.layout)

		if tc.vendordir == "" {
			tc.vendordir = "/vendor"
		}

		modulesDir := s.RootDir()
		uriModulesDir := uri.File(modulesDir)
		for _, cfg := range tc.configs {
			test.WriteFile(t, s.RootDir(), cfg.path,
				applyConfigTemplate(t, cfg.data.String(), uriModulesDir))

			git := sandbox.NewGit(t, filepath.Join(modulesDir, cfg.repo))
			git.CommitAll("files updated")
		}
		source := applyConfigTemplate(t, tc.source, uriModulesDir)
		return fixture{
			modsrc:        test.ParseSource(t, source),
			rootdir:       t.TempDir(),
			vendorDir:     project.NewPath(tc.vendordir),
			uriModulesDir: uriModulesDir,
		}
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			f := setup(t, tcase)
			got := download.Vendor(f.rootdir, f.vendorDir, f.modsrc, nil)
			want := applyReportTemplate(t, wantReport{
				Vendored: tcase.wantVendored,
				Ignored:  tcase.wantIgnored,
				Error:    tcase.wantError,
			}, string(f.uriModulesDir), f.vendorDir)

			assertVendorReport(t, want, got)
			checkWantedFiles(t, tcase, f.uriModulesDir, f.rootdir, f.vendorDir)
		})

		// Now we check that the same behavior is delivered via the vendor requests handler.
		t.Run(tcase.name+"/handling as event", func(t *testing.T) {
			t.Parallel()

			f := setup(t, tcase)

			events := make(chan event.VendorRequest)
			reports := download.HandleVendorRequests(f.rootdir, events, nil)
			events <- event.VendorRequest{
				VendorDir: f.vendorDir,
				Source:    f.modsrc,
			}
			close(events)

			got := <-reports
			want := applyReportTemplate(t, wantReport{
				Vendored: tcase.wantVendored,
				Ignored:  tcase.wantIgnored,
				Error:    tcase.wantError,
			}, string(f.uriModulesDir), f.vendorDir)

			assertVendorReport(t, want, got)
			checkWantedFiles(t, tcase, f.uriModulesDir, f.rootdir, f.vendorDir)

			if v, ok := <-reports; ok {
				t.Fatalf("unexpected report %v", v)
			}
		})
	}
}

func applyConfigTemplate(t *testing.T, input string, value interface{}) string {
	t.Helper()
	srctpl, err := template.New("template").Parse(input)
	assert.NoError(t, err)
	var buf bytes.Buffer
	err = srctpl.Execute(&buf, value)
	assert.NoError(t, err)
	return buf.String()
}

func applyReportTemplate(t *testing.T, r wantReport, value string, vendordir project.Path) download.Report {
	t.Helper()
	out := download.Report{
		Vendored: make(map[project.Path]download.Vendored),
		Error:    r.Error,
	}
	for _, vendored := range r.Vendored {
		rawSource := applyConfigTemplate(t, vendored, value)
		modsrc, err := tf.ParseSource(rawSource)
		assert.NoError(t, err)
		out.Vendored[modvendor.TargetDir(vendordir, modsrc)] = download.Vendored{
			Source: modsrc,
			Dir:    modvendor.TargetDir(vendordir, modsrc),
		}
	}
	for _, ignored := range r.Ignored {
		rawSource := applyConfigTemplate(t, ignored.RawSource, value)
		out.Ignored = append(out.Ignored, download.IgnoredVendor{
			RawSource: rawSource,
			Reason:    ignored.Error,
		})
	}
	return out
}

func evaluateWantedFiles(
	t *testing.T,
	wantFiles map[vendorPathSpec]fmt.Stringer,
	uriModulesDir uri.URI,
	rootdir string,
	vendordir project.Path,
) map[string]fmt.Stringer {
	t.Helper()
	evaluated := map[string]fmt.Stringer{}
	for pathSpec, expectedStringer := range wantFiles {
		pathSpecParts := strings.Split(string(pathSpec), "#")
		assert.EqualInts(t, len(pathSpecParts), 2)
		source := pathSpecParts[0]
		path := pathSpecParts[1]

		modsrc, err := tf.ParseSource(applyConfigTemplate(t, source, uriModulesDir))
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
	uriModulesDir uri.URI,
	rootdir string,
	vendordir project.Path,
) {
	t.Helper()

	wantFiles := evaluateWantedFiles(t, tc.wantFiles, uriModulesDir, rootdir, vendordir)
	absVendorDir := filepath.Join(rootdir, tc.vendordir)

	if _, err := os.Stat(absVendorDir); err != nil {
		if os.IsNotExist(err) {
			return
		}
		assert.Error(t, err)
	}

	err := filepath.Walk(absVendorDir, func(path string, _ fs.FileInfo, err error) error {
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
				t, filepath.Dir(path), tc.wantVendored, uriModulesDir, rootdir, vendordir,
			)
			want := applyConfigTemplate(t, expectedStringTemplate.String(), relVendoredPaths)
			got := string(test.ReadFile(t, filepath.Dir(path), filepath.Base(path)))
			assert.EqualStrings(t, want, got, "file %q mismatch", path)
		} else {
			// check the vendored file is the same as the one in the module dir.
			originalPath := modvendor.SourceDir(path, rootdir, vendordir)
			pathEnd := filepath.ToSlash(strings.TrimPrefix(originalPath, uriModulesDir.Filename()))

			originalPath = strings.TrimSuffix(filepath.ToSlash(originalPath), pathEnd)
			pathParts := strings.Split(pathEnd, "/")
			moduleName := pathParts[1]

			originalPath = filepath.Join(originalPath, moduleName, strings.Join(pathParts[3:], "/"))
			originalBytes, err := os.ReadFile(originalPath)
			assert.NoError(t, err)

			gotBytes, err := os.ReadFile(path)
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
	uriModulesDir uri.URI,
	rootdir string,
	vendordir project.Path,
) []string {
	t.Helper()
	// TODO(i4k): assumes files are always at the root of the module.
	relVendoredPaths := []string{}
	for _, vendored := range wantVendored {
		rawSource := applyConfigTemplate(t, vendored, uriModulesDir)
		modsrc, err := tf.ParseSource(rawSource)
		assert.NoError(t, err)
		relPath, err := filepath.Rel(relativeToDir,
			modvendor.AbsVendorDir(rootdir, vendordir, modsrc))
		assert.NoError(t, err)
		relVendoredPaths = append(relVendoredPaths, filepath.ToSlash(relPath))
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

	gitURI := uri.File(repoSandbox.RootDir())
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=%s", gitURI, ref))
	assert.NoError(t, err)

	const vendordir = "/dir/reftest/vendor"
	got := download.Vendor(rootdir, vendordir, source, nil)
	assertVendorReport(t, download.Report{
		Vendored: map[project.Path]download.Vendored{
			modvendor.TargetDir(vendordir, source): {
				Source: source,
				Dir:    modvendor.TargetDir(vendordir, source),
			},
		},
	}, got)

	cloneDir := modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[modvendor.TargetDir(vendordir, source)].Source)
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

	gitURI := uri.File(repoSandbox.RootDir())
	rootdir := t.TempDir()

	source := newSource(t, gitURI, ref)

	const vendordir = "/vendor"
	got := download.Vendor(rootdir, vendordir, source, nil)
	vendoredAt := modvendor.TargetDir(vendordir, source)
	assertVendorReport(t, download.Report{
		Vendored: map[project.Path]download.Vendored{
			vendoredAt: {
				Source: source,
				Dir:    modvendor.TargetDir(vendordir, source),
			},
		},
	}, got)

	cloneDir := got.Vendored[vendoredAt].Dir
	wantCloneDir := modvendor.TargetDir(vendordir, source)
	assert.EqualStrings(t, wantCloneDir.String(), cloneDir.String())

	absCloneDir := modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[vendoredAt].Source)
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
		URL:  string(gitURI),
		Ref:  newRef,
		Path: path,
	}
	got = download.Vendor(rootdir, vendordir, source, nil)

	wantCloneDir = modvendor.TargetDir(vendordir, source)
	newCloneDir := got.Vendored[wantCloneDir].Dir
	assert.EqualStrings(t, wantCloneDir.String(), newCloneDir.String())

	absCloneDir = modvendor.AbsVendorDir(rootdir, vendordir, got.Vendored[wantCloneDir].Source)
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

	gitURI := uri.File(s.RootDir())
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=main", gitURI))
	assert.NoError(t, err)

	const vendordir = "/vendor/fun"
	clonedir := modvendor.AbsVendorDir(rootdir, vendordir, source)
	test.MkdirAll(t, clonedir)
	got := download.Vendor(rootdir, vendordir, source, nil)
	want := download.Report{
		Ignored: []download.IgnoredVendor{
			{
				RawSource: source.Raw,
				Reason:    errors.E(download.ErrAlreadyVendored),
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
	gitURI := uri.File(s.RootDir())
	rootdir := t.TempDir()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s", gitURI))
	assert.NoError(t, err)
	report := download.Vendor(rootdir, "/vendor", source, nil)

	assertVendorReport(t, download.Report{
		Ignored: []download.IgnoredVendor{
			{
				RawSource: source.Raw,
				Reason:    errors.E(download.ErrModRefEmpty),
			},
		},
	}, report)
}

func TestModVendorVendorDirIsRelativeFails(t *testing.T) {
	const (
		path = "github.com/mineiros-io/example"
	)

	s := sandbox.New(t)
	gitURI := uri.File(s.RootDir())
	rootdir := t.TempDir()

	report := download.Vendor(rootdir, "../test", tf.Source{
		URL:  string(gitURI),
		Path: path,
		Ref:  "main",
	}, nil)

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

func assertVendorReport(t *testing.T, want, got download.Report) {
	t.Helper()

	assert.EqualInts(t, len(want.Vendored), len(got.Vendored),
		"number of vendored is different: want %s != got %s",
		want.Verbose(), got.Verbose())

	assert.EqualInts(t, len(want.Ignored), len(got.Ignored),
		"number of ignored is different: want %s != got %s",
		want.Verbose(), got.Verbose())

	for source, wantVendor := range want.Vendored {
		gotVendor, ok := got.Vendored[source]
		if !ok {
			t.Errorf("want vendor for source %q but got none", source)
			continue
		}
		if wantVendor != gotVendor {
			t.Errorf("vendored source: %s:\nwant:%#v\ngot:%#v\n",
				source, wantVendor, gotVendor)
		}
	}

	for i, wantIgnored := range want.Ignored {
		if wantIgnored.RawSource != got.Ignored[i].RawSource {
			t.Errorf("want.RawSource %v is different than %v",
				wantIgnored.RawSource, got.Ignored[i].RawSource)
		}
		assert.IsError(t, got.Ignored[i].Reason, wantIgnored.Reason)
	}

	errtest.Assert(t, got.Error, want.Error)
}

func newSource(t *testing.T, uri uri.URI, ref string) tf.Source {
	t.Helper()

	source, err := tf.ParseSource(fmt.Sprintf("git::%s?ref=%s", uri, ref))
	assert.NoError(t, err)
	return source
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

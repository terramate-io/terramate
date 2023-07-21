// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	stackpkg "github.com/terramate-io/terramate/stack"
	lstest "github.com/terramate-io/terramate/test/ls"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestInitialization(t *testing.T) {
	f := lstest.Setup(t)
	f.Editor.CheckInitialize(f.Sandbox.RootDir())
}

func TestDocumentOpen(t *testing.T) {
	f := lstest.Setup(t)

	stack := f.Sandbox.CreateStack("stack")
	f.Editor.CheckInitialize(f.Sandbox.RootDir())
	f.Editor.Open(fmt.Sprintf("stack/%s", stackpkg.DefaultFilename))
	r := <-f.Editor.Requests
	assert.EqualStrings(t, "textDocument/publishDiagnostics", r.Method(),
		"unexpected notification request")

	var params lsp.PublishDiagnosticsParams
	assert.NoError(t, json.Unmarshal(r.Params(), &params), "unmarshaling params")
	assert.EqualInts(t, 0, len(params.Diagnostics))
	assert.EqualStrings(t, filepath.Join(stack.Path(), stackpkg.DefaultFilename),
		params.URI.Filename())
}

func TestDocumentChange(t *testing.T) {
	t.Skip("not ready")

	type change struct {
		file string
		text string
	}
	type WantDiag struct {
		Range    lsp.Range
		Message  string
		Severity lsp.DiagnosticSeverity
	}
	type WantDiagParams struct {
		URI         lsp.URI
		Diagnostics []WantDiag
	}
	type testcase struct {
		name   string
		layout []string
		wrk    string // workspace
		change change
		want   []WantDiagParams
	}

	for _, tc := range []testcase{
		{
			name: "empty workspace and empty file change",
			change: change{
				file: "terramate.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace ok and empty file",
			layout: []string{
				"f:stack.tm:stack {}",
				"f:globals.tm:globals {}",
				"f:config.tm:terramate {}",
			},
			change: change{
				file: "empty.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI:         "config.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "empty.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "globals.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "stack.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace with issues and empty file change",
			layout: []string{
				"f:bug.tm:bug",
			},
			change: change{
				file: "terramate.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI: "bug.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 3,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace with issues and file with issues",
			layout: []string{
				"f:bug.tm:attr = 1",
			},
			change: change{
				file: "terramate.tm",
				text: "bug2",
			},
			want: []WantDiagParams{
				{
					URI: "bug.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 4,
								},
							},
						},
					},
				},
				{
					URI: "terramate.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 4,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "workspace with issues and file ok",
			layout: []string{
				"f:bug1.tm:bug1",
				"f:bug2.tm:terramate {test=1}",
			},
			change: change{
				file: "terramate.tm",
				text: "stack {}",
			},
			want: []WantDiagParams{
				{
					URI: "bug1.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 4,
								},
							},
						},
					},
				},
				{
					URI: "bug2.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Character: 11,
								},
								End: lsp.Position{
									Character: 15,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "multiple errors in the same file",
			change: change{
				file: "terramate.tm",
				text: `
terramate {
    a = 1
	config {
		b = 1
	}
	invalid {

	}
}
stack {
	n = "a"
}
`,
			},
			want: []WantDiagParams{
				{
					URI: "terramate.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      2,
									Character: 4,
								},
								End: lsp.Position{
									Line:      2,
									Character: 5,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      6,
									Character: 1,
								},
								End: lsp.Position{
									Line:      6,
									Character: 8,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      4,
									Character: 2,
								},
								End: lsp.Position{
									Line:      4,
									Character: 3,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      11,
									Character: 1,
								},
								End: lsp.Position{
									Line:      11,
									Character: 2,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple errors in the workspace",
			change: change{
				file: "terramate.tm",
				text: "",
			},
			layout: []string{
				`f:bug1.tm:terramate {
					a = 1
					b = 2
				}	
					`,
			},
			want: []WantDiagParams{
				{
					URI: "bug1.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      1,
									Character: 5,
								},
								End: lsp.Position{
									Line:      1,
									Character: 6,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      2,
									Character: 5,
								},
								End: lsp.Position{
									Line:      2,
									Character: 6,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace auto-detected and imports",
			layout: []string{
				"f:project/stack/stack.tm:stack {}",
				"f:project/modules/globals.tm:globals {}",
				`f:project/root.tm:
					terramate { 
						config { 
						
						}
					}
				`,
			},
			change: change{
				file: "project/stack/stack.tm",
				text: `
				stack {}
				import {
					source = "/modules/globals.tm"
				}
				`,
			},
			want: []WantDiagParams{
				{
					URI:         "project/stack/stack.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace set to rootdir and imports",
			wrk:  "project",
			layout: []string{
				"f:project/stack/stack.tm:stack {}",
				"f:project/modules/globals.tm:globals {}",
				`f:project/root.tm:
					terramate { 
						config { 
						
						}
					}
				`,
			},
			change: change{
				file: "project/stack/stack.tm",
				text: `
				stack {}
				import {
					source = "/modules/globals.tm"
				}
				`,
			},
			want: []WantDiagParams{
				{
					URI:         "project/stack/stack.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := lstest.Setup(t, tc.layout...)
			workspace := tc.wrk
			if workspace == "" {
				workspace = f.Sandbox.RootDir()
			}
			f.Editor.CheckInitialize(workspace)

			f.Editor.Change(tc.change.file, tc.change.text)
			for i := 0; i < len(tc.want); i++ {
				want := tc.want[i]

				// fix the wanted path as it depends on the sandbox root.
				want.URI = uri.File(filepath.Join(f.Sandbox.RootDir(), string(want.URI)))
				select {
				case gotReq := <-f.Editor.Requests:
					assert.EqualStrings(t, lsp.MethodTextDocumentPublishDiagnostics,
						gotReq.Method())

					var gotParams lsp.PublishDiagnosticsParams
					assert.NoError(t, json.Unmarshal(gotReq.Params(), &gotParams))
					assert.EqualInts(t,
						len(want.Diagnostics), len(gotParams.Diagnostics),
						"number of diagnostics mismatch: %s\n%s",
						cmp.Diff(gotParams, want), string(gotReq.Params()))

					assert.Partial(t, gotParams, want, "diagnostic mismatch")
				case <-time.After(10 * time.Millisecond):
					t.Fatalf("expected more requests: %s", cmp.Diff(nil, tc.want[i]))
				}
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test/sandbox"
	lsp "go.lsp.dev/protocol"
)

// TestLSPPositionAccuracy_GoToDefinition tests that LSP positions (0-indexed) are correctly
// calculated from HCL positions (1-indexed) for go-to-definition
func TestLSPPositionAccuracy_GoToDefinition(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name          string
		layout        []string
		file          string
		line          uint32 // LSP position (0-indexed)
		char          uint32 // LSP position (0-indexed)
		wantFile      string
		wantStartLine uint32 // Expected LSP Start position (0-indexed)
		wantStartChar uint32 // Expected LSP Start position (0-indexed)
		wantEndLine   uint32 // Expected LSP End position (0-indexed)
		wantEndChar   uint32 // Expected LSP End position (0-indexed)
		wantLocations int    // Number of expected locations (default 1)
	}

	testcases := []testcase{
		{
			name: "global attribute - exact position",
			layout: []string{
				`f:globals.tm:globals {
  region = "us-east-1"
}`,
				`f:stack.tm:stack {
  name = global.region
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          16, // on "region" in global.region
			wantFile:      "globals.tm",
			wantStartLine: 1,
			wantStartChar: 2, // "region" starts at column 2 (0-indexed)
			wantEndLine:   1,
			wantEndChar:   8, // "region" ends at column 8 (0-indexed)
		},
		{
			name: "labeled global attribute - exact position",
			layout: []string{
				`f:globals.tm:globals "gcp" "config" {
  region = "us-east-1"
}`,
				`f:stack.tm:stack {
  name = global.gcp.config.region
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          30, // on "region" in global.gcp.config.region
			wantFile:      "globals.tm",
			wantStartLine: 1,
			wantStartChar: 2, // "region" starts at column 2
			wantEndLine:   1,
			wantEndChar:   8, // "region" ends at column 8
		},
		{
			name: "labeled global - on label component",
			layout: []string{
				`f:globals.tm:globals "gcp" "config" {
  region = "us-east-1"
}`,
				`f:stack.tm:stack {
  name = global.gcp.config.region
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          16, // on "gcp" label in global.gcp.config.region
			wantFile:      "globals.tm",
			wantStartLine: 0,
			wantStartChar: 9, // "gcp" starts at column 9 (skips opening quote to point to 'g')
			wantEndLine:   0,
			wantEndChar:   12, // "gcp" ends at column 12 (skips closing quote, 0-indexed)
		},
		{
			name: "nested labeled global attribute - multi-level",
			layout: []string{
				`f:globals.tm:globals "a" "b" "c" {
  value = "test"
}`,
				`f:stack.tm:stack {
  name = global.a.b.c.value
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          24, // on "value" in global.a.b.c.value
			wantFile:      "globals.tm",
			wantStartLine: 1,
			wantStartChar: 2, // "value" starts at column 2
			wantEndLine:   1,
			wantEndChar:   7, // "value" ends at column 7
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srv := newTestServer(t, s.RootDir())

			fname := filepath.Join(s.RootDir(), tc.file)
			content := s.RootEntry().ReadFile(tc.file)

			locations, err := srv.findDefinitions(fname, content, tc.line, tc.char)
			assert.NoError(t, err)

			wantLocCount := tc.wantLocations
			if wantLocCount == 0 {
				wantLocCount = 1
			}

			if len(locations) != wantLocCount {
				t.Fatalf("expected %d location(s), got %d", wantLocCount, len(locations))
			}

			loc := locations[0]

			// Check file
			gotFile := filepath.Base(string(loc.URI.Filename()))
			if gotFile != tc.wantFile {
				t.Errorf("expected file %q, got %q", tc.wantFile, gotFile)
			}

			// Check exact LSP positions (0-indexed)
			if loc.Range.Start.Line != tc.wantStartLine {
				t.Errorf("Start.Line: expected %d, got %d", tc.wantStartLine, loc.Range.Start.Line)
			}
			if loc.Range.Start.Character != tc.wantStartChar {
				t.Errorf("Start.Character: expected %d, got %d (this should be 0-indexed)", tc.wantStartChar, loc.Range.Start.Character)
			}
			if loc.Range.End.Line != tc.wantEndLine {
				t.Errorf("End.Line: expected %d, got %d", tc.wantEndLine, loc.Range.End.Line)
			}
			if loc.Range.End.Character != tc.wantEndChar {
				t.Errorf("End.Character: expected %d, got %d (this should be 0-indexed)", tc.wantEndChar, loc.Range.End.Character)
			}
		})
	}
}

// TestLSPPositionAccuracy_Rename tests that rename ranges are correctly calculated
func TestLSPPositionAccuracy_Rename(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name          string
		layout        []string
		file          string
		line          uint32 // LSP position (0-indexed)
		char          uint32 // LSP position (0-indexed)
		newName       string
		wantStartLine uint32 // Expected Start.Line (0-indexed)
		wantStartChar uint32 // Expected Start.Character (0-indexed)
		wantEndLine   uint32 // Expected End.Line (0-indexed)
		wantEndChar   uint32 // Expected End.Character (0-indexed)
		wantEdits     int    // Number of expected edits across all files
	}

	testcases := []testcase{
		{
			name: "rename global attribute - exact range",
			layout: []string{
				`f:globals.tm:globals {
  my_var = "value"
}`,
				`f:stack.tm:stack {
  name = global.my_var
}`,
			},
			file:          "globals.tm",
			line:          1,
			char:          2, // on "my_var" definition
			newName:       "renamed_var",
			wantStartLine: 1,
			wantStartChar: 2, // Should start exactly at "m" in "my_var"
			wantEndLine:   1,
			wantEndChar:   8, // Should end exactly after "r" in "my_var"
			wantEdits:     2, // Definition + reference
		},
		{
			name: "rename global reference - exact range",
			layout: []string{
				`f:globals.tm:globals {
  project_id = "my-project"
}`,
				`f:stack.tm:stack {
  name = global.project_id
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          16, // on "project_id" in global.project_id
			newName:       "new_project",
			wantStartLine: 1,
			wantStartChar: 16, // Should start exactly at "p" in "project_id"
			wantEndLine:   1,
			wantEndChar:   26, // Should end exactly after "d" in "project_id"
			wantEdits:     2,  // Definition + reference
		},
		{
			name: "rename labeled global - exact range",
			layout: []string{
				`f:globals.tm:globals "gcp" {
  region = "us-east-1"
}`,
				`f:stack.tm:stack {
  name = global.gcp.region
}`,
			},
			file:          "stack.tm",
			line:          1,
			char:          23, // on "region" in global.gcp.region
			newName:       "zone",
			wantStartLine: 1,
			wantStartChar: 20, // Should start exactly at "r" in "region" (position 20)
			wantEndLine:   1,
			wantEndChar:   26, // Should end exactly after "n" in "region" (position 26)
			wantEdits:     2,  // Definition + reference
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srv := newTestServer(t, s.RootDir())

			fname := filepath.Join(s.RootDir(), tc.file)
			content := s.RootEntry().ReadFile(tc.file)

			// Test canRename - verifies the range we'll be renaming
			renameRange := srv.canRename(fname, content, tc.line, tc.char)
			assert.IsTrue(t, renameRange != nil, "should be able to rename")

			// Check exact LSP positions (0-indexed)
			if renameRange.Start.Line != tc.wantStartLine {
				t.Errorf("canRename Start.Line: expected %d, got %d", tc.wantStartLine, renameRange.Start.Line)
			}
			if renameRange.Start.Character != tc.wantStartChar {
				t.Errorf("canRename Start.Character: expected %d, got %d (this is the bug we fixed!)", tc.wantStartChar, renameRange.Start.Character)
			}
			if renameRange.End.Line != tc.wantEndLine {
				t.Errorf("canRename End.Line: expected %d, got %d", tc.wantEndLine, renameRange.End.Line)
			}
			if renameRange.End.Character != tc.wantEndChar {
				t.Errorf("canRename End.Character: expected %d, got %d", tc.wantEndChar, renameRange.End.Character)
			}

			// Test createRenameEdits - verifies all edit ranges
			workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, content, tc.line, tc.char, tc.newName)
			assert.NoError(t, err)
			assert.IsTrue(t, workspaceEdit != nil, "should create workspace edit")

			// Count total edits
			totalEdits := 0
			for _, edits := range workspaceEdit.Changes {
				totalEdits += len(edits)
			}

			if totalEdits != tc.wantEdits {
				t.Errorf("expected %d edits, got %d", tc.wantEdits, totalEdits)
			}

			// Verify all edits have correct new text
			for uri, edits := range workspaceEdit.Changes {
				for _, edit := range edits {
					if edit.NewText != tc.newName {
						t.Errorf("edit in %s: expected new text %q, got %q", uri, tc.newName, edit.NewText)
					}
					// Verify positions are 0-indexed (not negative, not absurdly large)
					if edit.Range.Start.Character > 1000 {
						t.Errorf("edit in %s: Start.Character %d seems wrong (should be 0-indexed)", uri, edit.Range.Start.Character)
					}
					if edit.Range.End.Character > 1000 {
						t.Errorf("edit in %s: End.Character %d seems wrong (should be 0-indexed)", uri, edit.Range.End.Character)
					}
				}
			}
		})
	}
}

// TestLSPPositionAccuracy_References tests that reference ranges are correctly calculated
func TestLSPPositionAccuracy_References(t *testing.T) {
	t.Parallel()

	type expectedLocation struct {
		file      string
		startLine uint32
		startChar uint32
		endLine   uint32
		endChar   uint32
	}

	type testcase struct {
		name     string
		layout   []string
		file     string
		line     uint32 // LSP position (0-indexed)
		char     uint32 // LSP position (0-indexed)
		wantLocs []expectedLocation
	}

	testcases := []testcase{
		{
			name: "find references - exact positions",
			layout: []string{
				`f:globals.tm:globals {
  my_var = "value"
}`,
				`f:stack.tm:stack {
  name = global.my_var
}`,
				`f:other.tm:stack {
  desc = global.my_var
}`,
			},
			file: "globals.tm",
			line: 1,
			char: 2, // on "my_var" definition
			wantLocs: []expectedLocation{
				{
					file:      "globals.tm",
					startLine: 1,
					startChar: 2,
					endLine:   1,
					endChar:   8,
				},
				{
					file:      "stack.tm",
					startLine: 1,
					startChar: 16,
					endLine:   1,
					endChar:   22,
				},
				{
					file:      "other.tm",
					startLine: 1,
					startChar: 16,
					endLine:   1,
					endChar:   22,
				},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srv := newTestServer(t, s.RootDir())

			fname := filepath.Join(s.RootDir(), tc.file)
			content := s.RootEntry().ReadFile(tc.file)

			locations, err := srv.findAllReferences(context.Background(), fname, content, tc.line, tc.char, true)
			assert.NoError(t, err)

			if len(locations) != len(tc.wantLocs) {
				t.Fatalf("expected %d locations, got %d", len(tc.wantLocs), len(locations))
			}

			// Create map for easier comparison (order may vary)
			locMap := make(map[string]lsp.Location)
			for _, loc := range locations {
				filename := filepath.Base(string(loc.URI.Filename()))
				locMap[filename] = loc
			}

			for _, want := range tc.wantLocs {
				loc, ok := locMap[want.file]
				if !ok {
					t.Errorf("expected location in file %q, not found", want.file)
					continue
				}

				// Check exact positions
				if loc.Range.Start.Line != want.startLine {
					t.Errorf("%s: Start.Line expected %d, got %d", want.file, want.startLine, loc.Range.Start.Line)
				}
				if loc.Range.Start.Character != want.startChar {
					t.Errorf("%s: Start.Character expected %d, got %d (0-indexed bug check!)", want.file, want.startChar, loc.Range.Start.Character)
				}
				if loc.Range.End.Line != want.endLine {
					t.Errorf("%s: End.Line expected %d, got %d", want.file, want.endLine, loc.Range.End.Line)
				}
				if loc.Range.End.Character != want.endChar {
					t.Errorf("%s: End.Character expected %d, got %d", want.file, want.endChar, loc.Range.End.Character)
				}
			}
		})
	}
}

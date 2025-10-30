// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestFindReferences(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name         string
		layout       []string
		file         string
		line         uint32
		char         uint32
		wantRefCount int
		includeDecl  bool
	}

	for _, tc := range []testcase{
		{
			name: "find all global variable references",
			layout: []string{
				`f:globals.tm:globals {
  my_var = "value"
}
`,
				`f:stack1.tm:stack {
  name = global.my_var
}
`,
				`f:stack2.tm:stack {
  description = global.my_var
}
`,
				`f:child/config.tm:globals {
  ref = global.my_var
}
`,
			},
			file:         "globals.tm",
			line:         1,
			char:         2, // on "my_var" definition
			wantRefCount: 4, // 1 definition + 3 references
			includeDecl:  true,
		},
		{
			name: "find references excluding declaration",
			layout: []string{
				`f:globals.tm:globals {
  test_var = "value"
}
`,
				`f:usage.tm:globals {
  ref1 = global.test_var
  ref2 = global.test_var
}
`,
			},
			file:         "globals.tm",
			line:         1,
			char:         2,
			wantRefCount: 2, // Only 2 references, not the definition
			includeDecl:  false,
		},
		{
			name: "find stack attribute references",
			layout: []string{
				`f:stack.tm:stack {
  name = "my-stack"
}

generate_hcl "test.hcl" {
  content {
    output "s1" {
      value = terramate.stack.name
    }
    output "s2" {
      value = terramate.stack.name
    }
    output "s3" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:         "stack.tm",
			line:         1,
			char:         2, // on "name" definition
			wantRefCount: 4, // 1 definition + 3 references
			includeDecl:  true,
		},
		{
			name: "find let variable references in generate block",
			layout: []string{
				`f:gen.tm:generate_hcl "test.hcl" {
  lets {
    my_let = "value"
  }
  
  content {
    a = let.my_let
    b = let.my_let
    c = let.my_let
  }
}
`,
			},
			file:         "gen.tm",
			line:         2,
			char:         4, // on "my_let" definition
			wantRefCount: 4, // 1 definition + 3 references
			includeDecl:  true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srv := newTestServer(t, s.RootDir())

			fname := filepath.Join(s.RootDir(), tc.file)
			content := test.ReadFile(t, s.RootDir(), tc.file)

			locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), tc.line, tc.char, tc.includeDecl)
			assert.NoError(t, err, "findAllReferences failed")

			assert.EqualInts(t, tc.wantRefCount, len(locations), "reference count mismatch")
		})
	}
}

func TestFindReferencesAcrossMultipleFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  environment = "production"
}
`,
		`f:stack1/config.tm:stack {
  name = global.environment
}
`,
		`f:stack2/config.tm:stack {
  description = global.environment
}
`,
		`f:stack3/config.tm:globals {
  env = global.environment
}
`,
	})

	srv := newTestServer(t, s.RootDir())

	fname := filepath.Join(s.RootDir(), "globals.tm")
	content := test.ReadFile(t, s.RootDir(), "globals.tm")

	// Find all references to "environment"
	locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
	assert.NoError(t, err)

	// Should find: 1 definition + 3 references = 4 locations
	assert.EqualInts(t, 4, len(locations), "should find all references across files")

	// Verify we found references in all files
	fileMap := make(map[string]bool)
	for _, loc := range locations {
		fileMap[loc.URI.Filename()] = true
	}

	assert.EqualInts(t, 4, len(fileMap), "references should be in 4 different files")
}

func TestFindReferencesEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("malformed HCL in reference file", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  my_var = "value"
}
`,
			`f:broken.tm:stack {
  name = global.my_var
  bad = "unclosed
}
`,
			`f:good.tm:stack {
  name = global.my_var
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Should find references in good files and skip broken ones
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
		assert.NoError(t, err)

		// Should find at least the definition and the good reference (broken file is skipped)
		assert.IsTrue(t, len(locations) >= 2, "should find references in parseable files")
	})

	t.Run("no references found", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  unused_var = "value"
}
`,
			`f:stack.tm:stack {
  name = "test"
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Should return only the definition when includeDecl is true
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
		assert.NoError(t, err)
		assert.EqualInts(t, 1, len(locations), "should find only the definition")

		// Should return empty when includeDecl is false
		locations, err = srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, false)
		assert.NoError(t, err)
		assert.EqualInts(t, 0, len(locations), "should find no references")
	})

	t.Run("references in deeply nested directories", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  deep_var = "value"
}
`,
			`f:a/ref1.tm:stack { name = global.deep_var }`,
			`f:a/b/ref2.tm:stack { name = global.deep_var }`,
			`f:a/b/c/ref3.tm:stack { name = global.deep_var }`,
			`f:a/b/c/d/ref4.tm:stack { name = global.deep_var }`,
			`f:a/b/c/d/e/ref5.tm:stack { name = global.deep_var }`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Should find all references across deeply nested directories
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
		assert.NoError(t, err)
		// 1 definition + 5 references = 6 total
		assert.EqualInts(t, 6, len(locations), "should find all references in deep nesting")
	})

	t.Run("let variable references only in same file", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:gen1.tm:generate_hcl "test.hcl" {
  lets {
    my_let = "value"
  }
  content {
    a = let.my_let
    b = let.my_let
  }
}
`,
			`f:gen2.tm:generate_hcl "other.hcl" {
  lets {
    different_let = "different"
  }
  content {
    c = let.different_let
  }
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "gen1.tm")
		content := test.ReadFile(t, s.RootDir(), "gen1.tm")

		// Should find references only in the same file (let is file-scoped)
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 2, 4, true)
		assert.NoError(t, err)
		// 1 definition + 2 references in same file = 3
		assert.EqualInts(t, 3, len(locations), "let references should be file-scoped")

		// All locations should be in gen1.tm
		for _, loc := range locations {
			gotFile := filepath.Base(loc.URI.Filename())
			assert.EqualStrings(t, "gen1.tm", gotFile, "all let references should be in same file")
		}
	})

	t.Run("let variables with same name in different files should not cross-reference", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:gen1.tm:generate_hcl "test1.hcl" {
  lets {
    shared_name = "value1"
  }
  content {
    a = let.shared_name
    b = let.shared_name
  }
}
`,
			`f:gen2.tm:generate_hcl "test2.hcl" {
  lets {
    shared_name = "value2"
  }
  content {
    c = let.shared_name
    d = let.shared_name
    e = let.shared_name
  }
}
`,
			`f:gen3.tm:generate_hcl "test3.hcl" {
  lets {
    shared_name = "value3"
  }
  content {
    f = let.shared_name
  }
}
`,
		})

		srv := newTestServer(t, s.RootDir())

		// Test finding references in gen1.tm
		fname1 := filepath.Join(s.RootDir(), "gen1.tm")
		content1 := test.ReadFile(t, s.RootDir(), "gen1.tm")

		locations1, err := srv.findAllReferences(context.Background(), fname1, []byte(content1), 2, 4, true)
		assert.NoError(t, err)
		// Should only find references in gen1.tm: 1 definition + 2 references = 3
		assert.EqualInts(t, 3, len(locations1), "should only find references in gen1.tm")

		for _, loc := range locations1 {
			gotFile := filepath.Base(loc.URI.Filename())
			assert.EqualStrings(t, "gen1.tm", gotFile, "should not find references from other files")
		}

		// Test finding references in gen2.tm
		fname2 := filepath.Join(s.RootDir(), "gen2.tm")
		content2 := test.ReadFile(t, s.RootDir(), "gen2.tm")

		locations2, err := srv.findAllReferences(context.Background(), fname2, []byte(content2), 2, 4, true)
		assert.NoError(t, err)
		// Should only find references in gen2.tm: 1 definition + 3 references = 4
		assert.EqualInts(t, 4, len(locations2), "should only find references in gen2.tm")

		for _, loc := range locations2 {
			gotFile := filepath.Base(loc.URI.Filename())
			assert.EqualStrings(t, "gen2.tm", gotFile, "should not find references from other files")
		}
	})

	t.Run("workspace with many files performance check", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)

		// Create a workspace with 100 files
		layout := []string{
			`f:globals.tm:globals {
  shared_var = "value"
}
`,
		}
		for i := 0; i < 100; i++ {
			filename := "stack" + string(rune('a'+i%26)) + string(rune('0'+(i/26))) + ".tm"
			layout = append(layout,
				`f:`+filename+`:stack { name = "stack" }`)
		}
		// Add a few actual references
		layout = append(layout, `f:ref1.tm:stack { name = global.shared_var }`)
		layout = append(layout, `f:ref2.tm:stack { desc = global.shared_var }`)

		s.BuildTree(layout)

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Should complete in reasonable time even with many files
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
		assert.NoError(t, err)
		// 1 definition + 2 references = 3
		assert.EqualInts(t, 3, len(locations), "should find all references")
	})

	t.Run("find references to map block label", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  map "deployment_totals" {
    for_each = []
  }
}
`,
			`f:stack1.tm:stack {
  name = global.deployment_totals
}
`,
			`f:stack2.tm:globals {
  ref = global.deployment_totals
}
`,
			`f:child/config.tm:stack {
  desc = global.deployment_totals
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Position cursor on the map block label "deployment_totals" at line 1, around column 10
		// Line 0: "globals {" - Line 1: "  map \"deployment_totals\" {"
		locations, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 10, true)
		assert.NoError(t, err)
		// 1 definition + 3 references = 4 locations
		assert.EqualInts(t, 4, len(locations), "should find all references to map block label")

		// Verify we found references in all files
		fileMap := make(map[string]bool)
		for _, loc := range locations {
			fileMap[loc.URI.Filename()] = true
		}
		assert.EqualInts(t, 4, len(fileMap), "references should be in 4 different files")
	})
}

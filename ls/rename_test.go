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

func TestPrepareRename(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name      string
		layout    []string
		file      string
		line      uint32
		char      uint32
		canRename bool
	}

	for _, tc := range []testcase{
		{
			name: "can rename global variable",
			layout: []string{
				`f:globals.tm:globals {
  my_var = "value"
}
`,
			},
			file:      "globals.tm",
			line:      1,
			char:      2, // on "my_var"
			canRename: true,
		},
		{
			name: "cannot rename stack metadata attribute",
			layout: []string{
				`f:stack.tm:stack {
  name = "test"
}

generate_hcl "test.hcl" {
  content {
    output "ref" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:      "stack.tm",
			line:      7,
			char:      33,    // on "name" in terramate.stack.name
			canRename: false, // terramate.stack.* metadata is protected
		},
		{
			name: "cannot rename stack attribute definition",
			layout: []string{
				`f:stack.tm:stack {
  name = "test"
}

generate_hcl "test.hcl" {
  content {
    output "ref" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:      "stack.tm",
			line:      1,
			char:      5,     // on "name" in definition: name = "test"
			canRename: false, // stack attributes are fixed metadata
		},
		{
			name: "cannot rename terramate built-in",
			layout: []string{
				`f:stack.tm:globals {
  path = terramate.path
}
`,
			},
			file:      "stack.tm",
			line:      1,
			char:      20, // on "path" in terramate.path
			canRename: false,
		},
		{
			name: "cannot rename string literal",
			layout: []string{
				`f:stack.tm:stack {
  name = "my-stack"
}
`,
			},
			file:      "stack.tm",
			line:      1,
			char:      11, // on "my-stack" string
			canRename: false,
		},
		{
			name: "cannot rename 'stack' part in terramate.stack.name",
			layout: []string{
				`f:stack.tm:stack {
  name = "test"
}

generate_hcl "test.hcl" {
  content {
    output "ref" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:      "stack.tm",
			line:      7,
			char:      26, // on "stack" in terramate.stack.name
			canRename: false,
		},
		{
			name: "cannot rename 'terramate' part in terramate.stack.name",
			layout: []string{
				`f:stack.tm:stack {
  name = "test"
}

generate_hcl "test.hcl" {
  content {
    output "ref" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:      "stack.tm",
			line:      7,
			char:      18, // on "terramate" in terramate.stack.name
			canRename: false,
		},
		{
			name: "cannot rename 'global' part in global.my_var",
			layout: []string{
				`f:globals.tm:globals {
  my_var = "value"
}
`,
				`f:stack.tm:stack {
  name = global.my_var
}
`,
			},
			file:      "stack.tm",
			line:      1,
			char:      9, // on "global" in global.my_var
			canRename: false,
		},
		{
			name: "cannot rename 'let' part in let.my_let",
			layout: []string{
				`f:gen.tm:generate_hcl "test.hcl" {
  lets {
    my_let = "value"
  }
  content {
    output "ref" {
      value = let.my_let
    }
  }
}
`,
			},
			file:      "gen.tm",
			line:      6,
			char:      14, // on "let" in let.my_let
			canRename: false,
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

			renameRange := srv.canRename(fname, []byte(content), tc.line, tc.char)

			if tc.canRename {
				assert.IsTrue(t, renameRange != nil, "expected to be able to rename")
			} else {
				assert.IsTrue(t, renameRange == nil, "expected to not be able to rename")
			}
		})
	}
}

func TestRenameGlobalVariable(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  old_name = "value"
}
`,
		`f:stack1.tm:stack {
  name = global.old_name
}
`,
		`f:stack2.tm:globals {
  ref = global.old_name
}
`,
	})

	srv := newTestServer(t, s.RootDir())

	fname := filepath.Join(s.RootDir(), "globals.tm")
	content := test.ReadFile(t, s.RootDir(), "globals.tm")

	// Rename old_name to new_name
	workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 2, "new_name")
	assert.NoError(t, err)
	assert.IsTrue(t, workspaceEdit != nil, "expected workspace edit")

	// Should have edits for 3 files (definition + 2 references)
	assert.IsTrue(t, len(workspaceEdit.Changes) >= 2, "should have edits in multiple files")

	// Verify each file has edits
	totalEdits := 0
	for _, edits := range workspaceEdit.Changes {
		totalEdits += len(edits)
		for _, edit := range edits {
			assert.EqualStrings(t, "new_name", edit.NewText, "edit should change to new_name")
		}
	}

	assert.IsTrue(t, totalEdits >= 3, "should have at least 3 edits (1 def + 2 refs)")
}

func TestCannotRenameStackMetadata(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
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
	})

	srv := newTestServer(t, s.RootDir())

	fname := filepath.Join(s.RootDir(), "stack.tm")
	content := test.ReadFile(t, s.RootDir(), "stack.tm")

	// Try to rename "name" in terramate.stack.name - should NOT be allowed
	workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 7, 33, "display_name")
	assert.NoError(t, err)
	assert.IsTrue(t, workspaceEdit == nil, "should NOT allow renaming terramate.stack.* metadata")
}

func TestCannotRenameStackAttributeDefinition(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
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
	})

	srv := newTestServer(t, s.RootDir())

	fname := filepath.Join(s.RootDir(), "stack.tm")
	content := test.ReadFile(t, s.RootDir(), "stack.tm")

	// Try to rename from the definition (not from a reference)
	// Position on "name" in the definition: name = "my-stack"
	// Stack attributes are fixed metadata and cannot be renamed
	workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 5, "display_name")
	assert.NoError(t, err)
	assert.IsTrue(t, workspaceEdit == nil, "should NOT allow renaming stack attribute definitions (fixed metadata)")
}

func TestRenameInvalidIdentifier(t *testing.T) {
	t.Parallel()

	invalidNames := []string{
		"123invalid", // starts with digit
		"my-var",     // contains hyphen
		"my var",     // contains space
		"my.var",     // contains dot
		"",           // empty
		// HCL reserved keywords
		"for",
		"in",
		"if",
		"else",
		"endif",
		"endfor",
		"null",
		"true",
		"false",
		// UTF-8 multi-byte characters (should be invalid as HCL only allows ASCII)
		"café",       // contains multi-byte UTF-8 character é
		"变量",         // Chinese characters
		"переменная", // Cyrillic characters
		"🚀rocket",    // starts with emoji (4-byte UTF-8)
		"var🎯",       // contains emoji in middle
		"αβγ",        // Greek letters
		"日本語",        // Japanese characters
	}

	for _, invalidName := range invalidNames {
		assert.IsTrue(t, !isValidIdentifier(invalidName),
			"'%s' should be invalid identifier", invalidName)
	}

	validNames := []string{
		"myVar",
		"my_var",
		"_private",
		"MyVar123",
		"var2",
		"For",      // Capital F - not a keyword
		"TRUE",     // Capital - not a keyword
		"if_else",  // Contains keyword but not exactly a keyword
		"for_loop", // Contains keyword but not exactly a keyword
	}

	for _, validName := range validNames {
		assert.IsTrue(t, isValidIdentifier(validName),
			"'%s' should be valid identifier", validName)
	}
}

func TestRenameGlobalInNestedDirectory(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:globals.tm:globals {
  parent_var = "parent"
}
`,
		`f:child/stack.tm:stack {
  name = global.parent_var
}
`,
		`f:child/nested/config.tm:globals {
  ref = global.parent_var
}
`,
	})

	srv := newTestServer(t, s.RootDir())

	// Trigger rename from the nested child directory
	fname := filepath.Join(s.RootDir(), "child/stack.tm")
	content := test.ReadFile(t, s.RootDir(), "child/stack.tm")

	// Rename parent_var to new_parent_var (position on "parent_var" in global.parent_var)
	workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 16, "new_parent_var")
	assert.NoError(t, err)
	assert.IsTrue(t, workspaceEdit != nil, "expected workspace edit")

	// Should rename the definition in parent and all references
	totalEdits := 0
	for _, edits := range workspaceEdit.Changes {
		totalEdits += len(edits)
		for _, edit := range edits {
			assert.EqualStrings(t, "new_parent_var", edit.NewText, "edit should change to new_parent_var")
		}
	}

	// Should have at least 3 edits: 1 definition + 2 references
	assert.IsTrue(t, totalEdits >= 3, "should rename definition in parent directory and all references")
}

func TestCannotRenameStackAttributeInNestedDirectory(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:parent/stack.tm:stack {
  name = "parent-stack"
  custom_attr = "value"
}
`,
		`f:parent/child/config.tm:generate_hcl "test.hcl" {
  content {
    output "attr" {
      value = terramate.stack.custom_attr
    }
  }
}
`,
	})

	srv := newTestServer(t, s.RootDir())

	// Trigger rename from the child directory
	fname := filepath.Join(s.RootDir(), "parent/child/config.tm")
	content := test.ReadFile(t, s.RootDir(), "parent/child/config.tm")

	// Try to rename custom_attr in terramate.stack.custom_attr - should NOT be allowed
	workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 3, 31, "renamed_attr")
	assert.NoError(t, err)
	assert.IsTrue(t, workspaceEdit == nil, "should NOT allow renaming terramate.stack.* attributes (even custom ones)")
}

func TestRenameEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("cannot rename in malformed HCL", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:broken.tm:globals {
  my_var = "unclosed
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "broken.tm")
		content := test.ReadFile(t, s.RootDir(), "broken.tm")

		// Should return nil for malformed file
		renameRange := srv.canRename(fname, []byte(content), 1, 2)
		assert.IsTrue(t, renameRange == nil, "cannot rename in malformed HCL")
	})

	t.Run("rename with special characters in new name", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  my_var = "value"
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "globals.tm")
		content := test.ReadFile(t, s.RootDir(), "globals.tm")

		// Try invalid names with special characters - they should all be rejected
		invalidNames := []string{"my-var", "my.var", "my var", "123start", ""}
		for _, invalidName := range invalidNames {
			workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 2, invalidName)
			assert.NoError(t, err)
			if workspaceEdit != nil {
				t.Errorf("should reject invalid identifier %q but got workspace edit", invalidName)
			}
		}
	})

	t.Run("rename affecting only specific file scope for let", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:gen1.tm:generate_hcl "test.hcl" {
  lets {
    my_let = "value1"
  }
  content {
    a = let.my_let
  }
}
`,
			`f:gen2.tm:generate_hcl "other.hcl" {
  lets {
    different_let = "value2"
  }
  content {
    b = let.different_let
  }
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "gen1.tm")
		content := test.ReadFile(t, s.RootDir(), "gen1.tm")

		// Rename in gen1.tm should not affect gen2.tm (different let variable)
		workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 2, 4, "renamed_let")
		assert.NoError(t, err)
		assert.IsTrue(t, workspaceEdit != nil, "should create workspace edit")

		// Check that only gen1.tm was modified
		gen1Modified := false
		gen2Modified := false
		for uri := range workspaceEdit.Changes {
			filename := filepath.Base(uri.Filename())
			if filename == "gen1.tm" {
				gen1Modified = true
			}
			if filename == "gen2.tm" {
				gen2Modified = true
			}
		}

		assert.IsTrue(t, gen1Modified, "gen1.tm should be modified")
		assert.IsTrue(t, !gen2Modified, "gen2.tm should NOT be modified (different let variable)")
	})

	t.Run("rename with no references only renames definition", func(t *testing.T) {
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

		// Should rename only the definition
		workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 2, "new_unused")
		assert.NoError(t, err)
		assert.IsTrue(t, workspaceEdit != nil, "should create workspace edit")

		totalEdits := 0
		for _, edits := range workspaceEdit.Changes {
			totalEdits += len(edits)
		}

		assert.EqualInts(t, 1, totalEdits, "should have exactly 1 edit (definition only)")
	})

	t.Run("rename across deeply nested structure", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  deep_var = "value"
}
`,
			`f:a/b/c/d/e/stack.tm:stack {
  name = global.deep_var
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "a/b/c/d/e/stack.tm")
		content := test.ReadFile(t, s.RootDir(), "a/b/c/d/e/stack.tm")

		// Should find and rename definition in ancestor directory
		workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 16, "renamed_deep")
		assert.NoError(t, err)
		assert.IsTrue(t, workspaceEdit != nil, "should create workspace edit")

		totalEdits := 0
		for _, edits := range workspaceEdit.Changes {
			totalEdits += len(edits)
		}

		assert.IsTrue(t, totalEdits >= 2, "should rename definition and reference")
	})

	t.Run("prepare rename on non-renameable symbol", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:config.tm:globals {
  path = terramate.path
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "config.tm")
		content := test.ReadFile(t, s.RootDir(), "config.tm")

		// Should not allow renaming built-in terramate.path
		renameRange := srv.canRename(fname, []byte(content), 1, 20)
		assert.IsTrue(t, renameRange == nil, "cannot rename built-in symbols")
	})
}

func TestRenameEnvironmentVariable(t *testing.T) {
	t.Parallel()

	t.Run("rename env variable defined in terramate.config.run.env", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:terramate.tm.hcl:terramate {
  config {
    run {
      env {
        MY_VAR = "value"
        OTHER_VAR = "other"
      }
    }
  }
}`,
			`f:stack.tm:stack {
  name = terramate.run.env.MY_VAR
}

generate_hcl "test.hcl" {
  content {
    value = terramate.run.env.MY_VAR
  }
}`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "stack.tm")
		content := test.ReadFile(t, s.RootDir(), "stack.tm")

		// Rename MY_VAR to RENAMED_VAR
		// Position on MY_VAR in line: name = terramate.run.env.MY_VAR
		workspaceEdit, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 27, "RENAMED_VAR")
		assert.NoError(t, err)
		assert.IsTrue(t, workspaceEdit != nil, "should create workspace edit for env var")

		// Should rename definition and both references
		totalEdits := 0
		for _, edits := range workspaceEdit.Changes {
			totalEdits += len(edits)
		}

		assert.IsTrue(t, totalEdits == 3, "should rename definition (terramate.tm.hcl) and 2 references (stack.tm)")
	})

	t.Run("cannot rename built-in terramate variables", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:stack.tm:stack {
  name = terramate.path
}`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "stack.tm")
		content := test.ReadFile(t, s.RootDir(), "stack.tm")

		// Should not allow renaming terramate.path
		renameRange := srv.canRename(fname, []byte(content), 1, 20)
		assert.IsTrue(t, renameRange == nil, "cannot rename built-in terramate.* variables")
	})

	t.Run("cannot rename terramate.stack metadata attributes", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:stack.tm:stack {
  name = "my-stack"
  description = "test"
}

globals {
  stack_name = terramate.stack.name
  stack_id = terramate.stack.id
  stack_path = terramate.stack.path.absolute
}`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "stack.tm")
		content := test.ReadFile(t, s.RootDir(), "stack.tm")

		// Try to rename "name" in terramate.stack.name (line 5, position on "name")
		renameRange := srv.canRename(fname, []byte(content), 5, 26)
		assert.IsTrue(t, renameRange == nil, "cannot rename terramate.stack.name")

		// Try to rename "id" in terramate.stack.id
		renameRange = srv.canRename(fname, []byte(content), 6, 24)
		assert.IsTrue(t, renameRange == nil, "cannot rename terramate.stack.id")

		// Try to rename "absolute" in terramate.stack.path.absolute
		renameRange = srv.canRename(fname, []byte(content), 7, 42)
		assert.IsTrue(t, renameRange == nil, "cannot rename terramate.stack.path.absolute")
	})
}

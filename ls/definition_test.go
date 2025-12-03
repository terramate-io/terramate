// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/jsonrpc2"
)

// testingTB is the common interface for testing.T and testing.B
type testingTB interface {
	Helper()
	Cleanup(func())
	Failed() bool
	Logf(format string, args ...interface{})
}

// newTestServer creates a new language server for testing
func newTestServer(t testingTB, workspaces ...string) *Server {
	t.Helper()

	conn := &testConn{}

	// Create a logger that works with both T and B
	var logger zerolog.Logger
	if testT, ok := t.(*testing.T); ok {
		logger = zerolog.New(zerolog.NewTestWriter(testT))
	} else if testB, ok := t.(*testing.B); ok {
		logger = zerolog.New(zerolog.NewTestWriter(testB))
	} else {
		logger = zerolog.Nop()
	}

	srv := NewServer(conn, WithLogger(logger))
	srv.workspaces = workspaces

	return srv
}

// testConn is a test implementation of jsonrpc2.Conn
type testConn struct{}

func (c *testConn) Go(_ context.Context, _ jsonrpc2.Handler) {
	// No-op for testing
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) Notify(_ context.Context, _ string, _ interface{}) error {
	return nil
}

func (c *testConn) Call(_ context.Context, _ string, _, _ interface{}) (jsonrpc2.ID, error) {
	return jsonrpc2.NewNumberID(0), nil
}

func (c *testConn) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (c *testConn) Err() error {
	return nil
}

func TestFindDefinition(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name     string
		layout   []string
		file     string
		line     uint32
		char     uint32
		wantFile string
		wantLine uint32
	}

	for _, tc := range []testcase{
		{
			name: "find global definition in same file",
			layout: []string{
				`f:config.tm:globals {
  my_var = "value"
}

stack {
  name = global.my_var
}
`,
			},
			file:     "config.tm",
			line:     5,  // line with "name = global.my_var"
			char:     16, // position on "my_var" in "global.my_var"
			wantFile: "config.tm",
			wantLine: 1, // line with "my_var = "value""
		},
		{
			name: "find global definition in parent directory",
			layout: []string{
				`f:globals.tm:globals {
  parent_var = "parent"
}
`,
				`f:child/stack.tm:stack {
  name = global.parent_var
}
`,
			},
			file:     "child/stack.tm",
			line:     1,
			char:     16, // position on "parent_var"
			wantFile: "globals.tm",
			wantLine: 1,
		},
		{
			name: "find map block definition",
			layout: []string{
				`f:globals.tm:globals {
  simple_var = "value"
  
  map "deployment_totals" {
    for_each = [
      { env = "dev", cost = 100 },
    ]
    key = element.new.env
    value {
      total = element.new.cost
    }
  }
}
`,
				`f:usage.tm:globals {
  totals = global.deployment_totals
}
`,
			},
			file:     "usage.tm",
			line:     1,
			char:     18, // position on "deployment_totals"
			wantFile: "globals.tm",
			wantLine: 3, // line with map "deployment_totals"
		},
		{
			name: "find labeled globals definition (deep nesting)",
			layout: []string{
				`f:config.tm:globals "gclz_config" "terraform" "providers" "google" "config" {
  region = "europe-west1"
  project = "my-project"
}
`,
				`f:stack.tm:globals {
  gclz_region = global.gclz_config.terraform.providers.google.config.region
}
`,
			},
			file:     "stack.tm",
			line:     1,
			char:     70, // position on "region" at end of long path
			wantFile: "config.tm",
			wantLine: 1, // line with region = "europe-west1"
		},
		{
			name: "find labeled globals with multiple attributes",
			layout: []string{
				`f:config.tm:globals "settings" "database" {
  host = "localhost"
  port = 5432
  name = "mydb"
}
`,
				`f:usage.tm:globals {
  db_host = global.settings.database.host
  db_port = global.settings.database.port
}
`,
			},
			file:     "usage.tm",
			line:     2,
			char:     40, // position on "port"
			wantFile: "config.tm",
			wantLine: 2, // line with port = 5432
		},
		{
			name: "find simple global defined with complex expression",
			layout: []string{
				`f:globals.tm:globals {
  gclz_project_id = tm_ternary(
    tm_can(tm_regex("^/stacks/", terramate.stack.path.absolute)),
    "project-123",
    "default-project"
  )
}
`,
				`f:usage.tm:globals {
  project = global.gclz_project_id
}
`,
			},
			file:     "usage.tm",
			line:     1,
			char:     18, // position on "gclz_project_id"
			wantFile: "globals.tm",
			wantLine: 1, // line with gclz_project_id definition
		},
		{
			name: "find nested object in unlabeled globals (like gclz_meta.env)",
			layout: []string{
				`f:meta.tm:globals {
  gclz_meta = {
    env = "production"
    region = "us-east-1"
  }
}
`,
				`f:usage.tm:globals {
  environment = global.gclz_meta.env
}
`,
			},
			file:     "usage.tm",
			line:     1,
			char:     26, // position on "gclz_meta"
			wantFile: "meta.tm",
			wantLine: 1, // line with gclz_meta = { ... } (root object)
		},
		{
			name: "find simple global across files (different .tm files in same dir)",
			layout: []string{
				`f:root.tm:globals {
  gclz_project_id = "project-123"
}
`,
				`f:config.tm:globals "gclz_config" "terraform" "locals" {
  gclz_project_id = global.gclz_project_id
}
`,
			},
			file:     "config.tm",
			line:     1,
			char:     24, // position on "gclz_project_id" in reference
			wantFile: "root.tm",
			wantLine: 1, // line with gclz_project_id = "project-123"
		},
		{
			name: "find let definition in generate_hcl block",
			layout: []string{
				`f:gen.tm:generate_hcl "test.hcl" {
  lets {
    my_let = "value"
  }
  
  content {
    data = let.my_let
  }
}
`,
			},
			file:     "gen.tm",
			line:     6,
			char:     15, // position on "my_let" in "let.my_let"
			wantFile: "gen.tm",
			wantLine: 2,
		},
		{
			name: "find stack attribute definition",
			layout: []string{
				`f:stack.tm:stack {
  name        = "my-stack"
  description = "test stack"
  id          = "abc123"
}

generate_hcl "test.hcl" {
  content {
    output "name" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:     "stack.tm",
			line:     9,
			char:     33, // position on "name" in "terramate.stack.name"
			wantFile: "stack.tm",
			wantLine: 1,
		},
		{
			name: "find stack attribute via terramate.stack",
			layout: []string{
				`f:stack.tm:stack {
  name = "my-stack"
  id   = "abc123"
}

generate_hcl "test.hcl" {
  content {
    output "name" {
      value = terramate.stack.name
    }
  }
}
`,
			},
			file:     "stack.tm",
			line:     8,
			char:     33, // position on "name" in "terramate.stack.name"
			wantFile: "stack.tm",
			wantLine: 1,
		},
		{
			name: "find stack description in parent directory",
			layout: []string{
				`f:stack.tm:stack {
  name        = "parent-stack"
  description = "parent desc"
}
`,
				`f:child/config.tm:generate_hcl "test.hcl" {
  content {
    output "desc" {
      value = terramate.stack.description
    }
  }
}
`,
			},
			file:     "child/config.tm",
			line:     3,
			char:     34, // position on "description" in terramate.stack.description
			wantFile: "stack.tm",
			wantLine: 2,
		},
		{
			name: "find env variable definition in same file",
			layout: []string{
				`f:stack.tm:stack {
  name = "test-stack"
}

terramate {
  config {
    run {
      env {
        FOO = "BAR"
      }
    }
  }
}

globals {
  foo = terramate.run.env.FOO
}
`,
			},
			file:     "stack.tm",
			line:     15, // line with "foo = terramate.run.env.FOO"
			char:     26, // position on "FOO" in "terramate.run.env.FOO"
			wantFile: "stack.tm",
			wantLine: 8, // line with "FOO = "BAR""
		},
		{
			name: "find env variable definition in parent directory",
			layout: []string{
				`f:terramate.tm.hcl:terramate {
  config {
    run {
      env {
        PARENT_VAR = "parent"
      }
    }
  }
}
`,
				`f:child/stack.tm:stack {
  name = "child-stack"
}

globals {
  var = terramate.run.env.PARENT_VAR
}
`,
			},
			file:     "child/stack.tm",
			line:     5,  // line with "var = terramate.run.env.PARENT_VAR"
			char:     30, // position on "PARENT_VAR"
			wantFile: "terramate.tm.hcl",
			wantLine: 4, // line with "PARENT_VAR = "parent""
		},
		{
			name: "find all env variable definitions when defined at multiple levels",
			layout: []string{
				`f:terramate.tm.hcl:terramate {
  config {
    run {
      env {
        FOO = "project-wide"
      }
    }
  }
}
`,
				`f:child/stack.tm:stack {
  name = "child-stack"
}

terramate {
  config {
    run {
      env {
        FOO = "stack-override"
      }
    }
  }
}

globals {
  var = terramate.run.env.FOO
}
`,
			},
			file:     "child/stack.tm",
			line:     15, // line with "var = terramate.run.env.FOO"
			char:     26, // position on "FOO"
			wantFile: "", // We'll check multiple locations below
			wantLine: 0,
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

			locations, err := srv.findDefinitions(fname, []byte(content), tc.line, tc.char)
			assert.NoError(t, err, "findDefinitions failed")

			// Special handling for the multi-level env variable test
			if tc.name == "find all env variable definitions when defined at multiple levels" {
				// Should find 2 definitions: one in terramate.tm.hcl and one in child/stack.tm
				assert.IsTrue(t, len(locations) == 2, "expected to find 2 definitions (project + stack level)")

				// Check we have both files
				gotFiles := make(map[string]bool)
				for _, loc := range locations {
					gotFiles[loc.URI.Filename()] = true
				}

				wantTerramateConfig := filepath.Join(s.RootDir(), "terramate.tm.hcl")
				wantStackFile := filepath.Join(s.RootDir(), "child/stack.tm")

				assert.IsTrue(t, gotFiles[wantTerramateConfig], "should find definition in terramate.tm.hcl")
				assert.IsTrue(t, gotFiles[wantStackFile], "should find definition in child/stack.tm")
				return
			}

			if tc.wantFile == "" {
				// Expected no definition found
				if len(locations) > 0 {
					t.Fatalf("expected no definition, but found one at %v", locations[0])
				}
				return
			}

			assert.IsTrue(t, len(locations) > 0, "expected to find definition")
			location := locations[0]

			wantPath := filepath.Join(s.RootDir(), tc.wantFile)
			gotPath := location.URI.Filename()

			assert.EqualStrings(t, wantPath, gotPath, "definition file mismatch")
			assert.EqualInts(t, int(tc.wantLine), int(location.Range.Start.Line), "definition line mismatch")
		})
	}
}

func TestFindDefinitionNoSymbol(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:config.tm:stack {
  name = "test"
}
`,
	})

	srv := newTestServer(t, s.RootDir())

	fname := filepath.Join(s.RootDir(), "config.tm")
	content := test.ReadFile(t, s.RootDir(), "config.tm")

	// Position on a string literal (not a symbol reference)
	locations, err := srv.findDefinitions(fname, []byte(content), 1, 11)
	assert.NoError(t, err)
	assert.IsTrue(t, len(locations) == 0, "should not find definition for string literal")
}

func TestFindImportDefinition(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name     string
		layout   []string
		file     string
		line     uint32
		char     uint32
		wantFile string
	}

	for _, tc := range []testcase{
		{
			name: "navigate to imported file with absolute path",
			layout: []string{
				`f:shared/common.tm:globals {
  shared_var = "shared"
}
`,
				`f:stack/config.tm:import {
  source = "/shared/common.tm"
}

stack {
  name = "test"
}
`,
			},
			file:     "stack/config.tm",
			line:     1,
			char:     14, // position inside "/shared/common.tm" string
			wantFile: "shared/common.tm",
		},
		{
			name: "navigate to imported file with relative path",
			layout: []string{
				`f:shared/globals.tm:globals {
  env = "production"
}
`,
				`f:stack/config.tm:import {
  source = "../shared/globals.tm"
}

stack {
  name = "test"
}
`,
			},
			file:     "stack/config.tm",
			line:     1,
			char:     15, // position inside "../shared/globals.tm" string
			wantFile: "shared/globals.tm",
		},
		{
			name: "navigate to nested import path (like test-stack)",
			layout: []string{
				`f:test-stack/imports/shared-globals.tm:globals {
  shared_var = "shared"
}
`,
				`f:test-stack/stack.tm:import {
  source = "/test-stack/imports/shared-globals.tm"
}

stack {
  name = "test"
}
`,
			},
			file:     "test-stack/stack.tm",
			line:     1,
			char:     12, // position on "source" attribute
			wantFile: "test-stack/imports/shared-globals.tm",
		},
		{
			name: "navigate from anywhere on source line",
			layout: []string{
				`f:common.tm:globals {
  var = "value"
}
`,
				`f:config.tm:import {
  source = "/common.tm"
}
`,
			},
			file:     "config.tm",
			line:     1,
			char:     3, // position on "source" keyword itself
			wantFile: "common.tm",
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

			locations, err := srv.findDefinitions(fname, []byte(content), tc.line, tc.char)
			assert.NoError(t, err, "findDefinitions failed")

			assert.IsTrue(t, len(locations) > 0, "expected to find import target")
			location := locations[0]

			wantPath := filepath.Join(s.RootDir(), tc.wantFile)
			gotPath := location.URI.Filename()

			assert.EqualStrings(t, wantPath, gotPath, "import target file mismatch")
		})
	}
}

func TestFindDefinitionEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("malformed HCL file", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:broken.tm:globals {
  my_var = "unclosed string
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "broken.tm")
		content := test.ReadFile(t, s.RootDir(), "broken.tm")

		// Should handle gracefully without panicking
		locations, _ := srv.findDefinitions(fname, []byte(content), 1, 2)
		// We expect empty slice for unparseable file
		assert.IsTrue(t, len(locations) == 0, "should return empty slice for malformed file")
	})

	t.Run("non-existent import file", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:config.tm:import {
  source = "/non-existent/file.tm"
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "config.tm")
		content := test.ReadFile(t, s.RootDir(), "config.tm")

		// Should handle gracefully
		locations, err := srv.findDefinitions(fname, []byte(content), 1, 12)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) == 0, "should return empty slice for non-existent import")
	})

	t.Run("glob pattern in import", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:config.tm:import {
  source = "/shared/*.tm"
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "config.tm")
		content := test.ReadFile(t, s.RootDir(), "config.tm")

		// Glob patterns should return empty slice (can't navigate to multiple files)
		locations, err := srv.findDefinitions(fname, []byte(content), 1, 12)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) == 0, "should return empty slice for glob patterns")
	})

	t.Run("deep directory nesting", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:globals.tm:globals {
  deep_var = "value"
}
`,
			`f:a/b/c/d/e/f/g/h/i/j/config.tm:stack {
  name = global.deep_var
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "a/b/c/d/e/f/g/h/i/j/config.tm")
		content := test.ReadFile(t, s.RootDir(), "a/b/c/d/e/f/g/h/i/j/config.tm")

		// Should find definition even with deep nesting
		locations, err := srv.findDefinitions(fname, []byte(content), 1, 14)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) > 0, "should find definition in deeply nested structure")

		location := locations[0]
		wantPath := filepath.Join(s.RootDir(), "globals.tm")
		assert.EqualStrings(t, wantPath, location.URI.Filename())
	})

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:empty.tm:`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "empty.tm")
		content := test.ReadFile(t, s.RootDir(), "empty.tm")

		// Should handle empty file gracefully
		locations, err := srv.findDefinitions(fname, []byte(content), 0, 0)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) == 0, "should return empty slice for empty file")
	})

	t.Run("undefined variable reference", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:config.tm:stack {
  name = global.undefined_var
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "config.tm")
		content := test.ReadFile(t, s.RootDir(), "config.tm")

		// Should return empty slice for undefined variable
		locations, err := srv.findDefinitions(fname, []byte(content), 1, 16)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) == 0, "should return empty slice for undefined variable")
	})

	t.Run("circular import scenario", func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{
			`f:a.tm:import {
  source = "/b.tm"
}

globals {
  var_a = "a"
}
`,
			`f:b.tm:import {
  source = "/a.tm"
}

globals {
  var_b = "b"
}
`,
		})

		srv := newTestServer(t, s.RootDir())
		fname := filepath.Join(s.RootDir(), "a.tm")
		content := test.ReadFile(t, s.RootDir(), "a.tm")

		// Should navigate to b.tm without infinite loop
		locations, err := srv.findDefinitions(fname, []byte(content), 1, 12)
		assert.NoError(t, err)
		assert.IsTrue(t, len(locations) > 0, "should handle circular imports")

		location := locations[0]
		wantPath := filepath.Join(s.RootDir(), "b.tm")
		assert.EqualStrings(t, wantPath, location.URI.Filename())
	})
}

func TestFindStackDependencyDefinition(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name     string
		layout   []string
		file     string
		line     uint32
		char     uint32
		wantFile string
	}

	for _, tc := range []testcase{
		{
			name: "navigate to stack via after dependency",
			layout: []string{
				`f:database/stack.tm:stack {
  name = "database"
  id   = "db-123"
}
`,
				`f:app/stack.tm:stack {
  name  = "app"
  id    = "app-456"
  after = ["/database"]
}
`,
			},
			file:     "app/stack.tm",
			line:     3,
			char:     13, // position inside "/database" string
			wantFile: "database/stack.tm",
		},
		{
			name: "navigate to stack via before dependency",
			layout: []string{
				`f:frontend/stack.tm:stack {
  name = "frontend"
  id   = "fe-789"
}
`,
				`f:backend/stack.tm:stack {
  name   = "backend"
  id     = "be-456"
  before = ["/frontend"]
}
`,
			},
			file:     "backend/stack.tm",
			line:     3,
			char:     14, // position inside "/frontend" string
			wantFile: "frontend/stack.tm",
		},
		{
			name: "navigate to stack via wants dependency",
			layout: []string{
				`f:monitoring/stack.tm:stack {
  name = "monitoring"
}
`,
				`f:service/stack.tm:stack {
  name  = "service"
  wants = ["/monitoring"]
}
`,
			},
			file:     "service/stack.tm",
			line:     2,
			char:     13, // position inside "/monitoring" string
			wantFile: "monitoring/stack.tm",
		},
		{
			name: "navigate to stack via wanted_by dependency",
			layout: []string{
				`f:api/stack.tm:stack {
  name = "api"
}
`,
				`f:shared-lib/stack.tm:stack {
  name      = "shared-lib"
  wanted_by = ["/api"]
}
`,
			},
			file:     "shared-lib/stack.tm",
			line:     2,
			char:     16, // position inside "/api" string
			wantFile: "api/stack.tm",
		},
		{
			name: "navigate to stack with multiple dependencies",
			layout: []string{
				`f:vpc/stack.tm:stack {
  name = "vpc"
}
`,
				`f:security/stack.tm:stack {
  name = "security"
}
`,
				`f:app/stack.tm:stack {
  name  = "app"
  after = [
    "/vpc",
    "/security",
  ]
}
`,
			},
			file:     "app/stack.tm",
			line:     3,
			char:     7, // position inside "/vpc" string
			wantFile: "vpc/stack.tm",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			srv := newTestServer(t, s.RootDir())

			fname := filepath.Join(s.RootDir(), tc.file)
			content := test.ReadFile(t, s.RootDir(), tc.file)

			locations, err := srv.findDefinitions(fname, []byte(content), tc.line, tc.char)
			assert.NoError(t, err, "findDefinitions failed")

			assert.IsTrue(t, len(locations) > 0, "expected to find stack dependency")

			location := locations[0]
			wantPath := filepath.Join(s.RootDir(), tc.wantFile)
			gotPath := location.URI.Filename()

			assert.EqualStrings(t, wantPath, gotPath, "stack dependency file mismatch")
		})
	}
}

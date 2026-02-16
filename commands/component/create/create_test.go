// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package create

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl"

	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// mockCLI implements commands.CLI for testing
type mockCLI struct {
	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
	cliConfig  cliconfig.Config
	stderr     io.Writer
	stdin      io.Reader
	stdout     io.Writer
}

func (m *mockCLI) WorkingDir() string {
	return m.workingDir
}

func (m *mockCLI) Engine() *engine.Engine {
	return m.engine
}

func (m *mockCLI) Printers() printer.Printers {
	return m.printers
}

func (m *mockCLI) Config() cliconfig.Config {
	return m.cliConfig
}

func (m *mockCLI) PrettyProduct() string {
	return "Terramate Catalyst"
}

func (m *mockCLI) Product() string {
	return "terramate-catalyst"
}

func (m *mockCLI) Stderr() io.Writer {
	if m.stderr != nil {
		return m.stderr
	}
	return os.Stderr
}

func (m *mockCLI) Stdin() io.Reader {
	if m.stdin != nil {
		return m.stdin
	}
	return os.Stdin
}

func (m *mockCLI) Stdout() io.Writer {
	if m.stdout != nil {
		return m.stdout
	}
	return os.Stdout
}

func (m *mockCLI) Version() string {
	return "dev"
}

func (m *mockCLI) Reload(_ context.Context) error {
	return nil
}

// newEngineForTest creates an engine for testing by loading the root config
// and creating an engine using reflection/unsafe to set the private config field.
// The sandbox must have a terramate.tm.hcl file.
func newEngineForTest(t *testing.T, s sandbox.S) *engine.Engine {
	t.Helper()

	// Create a minimal terramate config if it doesn't exist
	terramateFile := filepath.Join(s.RootDir(), "terramate.tm.hcl")
	if _, err := os.Stat(terramateFile); os.IsNotExist(err) {
		err := os.WriteFile(terramateFile, []byte(`terramate {}`), 0644)
		assert.NoError(t, err)
	}

	// Load root config
	rootCfg, err := config.LoadRoot(s.RootDir(), false,
		hcl.WithMergedLabelsBlockHandlers(hcl.DefaultMergedLabelsBlockHandlers()...),
		hcl.WithUnmergedBlockHandlers(hcl.DefaultUnmergedBlockParsers()...),
	)
	assert.NoError(t, err)

	// Create a new engine instance using reflection
	engineType := reflect.TypeOf((*engine.Engine)(nil)).Elem()
	engineValue := reflect.New(engineType)
	enginePtr := engineValue.Interface().(*engine.Engine)
	engineElem := engineValue.Elem()

	// The engine has a "project" field of type *engine.Project
	// We need to create a project and set it in the engine
	projectField := engineElem.FieldByName("project")
	if !projectField.IsValid() {
		t.Fatalf("could not find 'project' field in engine.Engine")
	}

	// Create a project instance
	projectType := reflect.TypeOf((*engine.Project)(nil)).Elem()
	projectValue := reflect.New(projectType)
	projectPtr := projectValue.Interface().(*engine.Project)
	projectElem := projectValue.Elem()

	gitWrapper, err := git.WithConfig(git.Config{
		WorkingDir: s.RootDir(),
	})
	assert.NoError(t, err)

	gitField := projectElem.FieldByName("Git")
	if !gitField.IsValid() {
		t.Fatalf("could not find 'Git' field in engine.Project")
	}
	wrapperField := gitField.FieldByName("Wrapper")
	if !wrapperField.IsValid() {
		t.Fatalf("could not find 'Git.Wrapper' field in engine.Project")
	}
	wrapperField.Set(reflect.ValueOf(gitWrapper))

	// Find the root config field in the project
	rootCfgType := reflect.TypeOf((*config.Root)(nil))
	for i := 0; i < projectElem.NumField(); i++ {
		field := projectElem.Field(i)
		fieldType := projectElem.Type().Field(i)

		if fieldType.Type == rootCfgType {
			// Found the config field in project - set it
			fieldPtr := unsafe.Pointer(field.UnsafeAddr())
			*(**config.Root)(fieldPtr) = rootCfg
			// Now set the project in the engine
			projectFieldPtr := unsafe.Pointer(projectField.UnsafeAddr())
			*(**engine.Project)(projectFieldPtr) = projectPtr
			return enginePtr
		}
	}

	// Try common field names in project
	fieldNames := []string{"root", "cfg", "config", "rootCfg"}
	for _, name := range fieldNames {
		field := projectElem.FieldByName(name)
		if field.IsValid() && field.Type() == rootCfgType {
			fieldPtr := unsafe.Pointer(field.UnsafeAddr())
			*(**config.Root)(fieldPtr) = rootCfg
			projectFieldPtr := unsafe.Pointer(projectField.UnsafeAddr())
			*(**engine.Project)(projectFieldPtr) = projectPtr
			return enginePtr
		}
	}

	t.Fatalf("could not create engine for testing - could not find config field in engine.Project (checked %d fields)", projectElem.NumField())
	return nil
}

func TestPathResolution(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name        string
		path        string
		setup       func(*testing.T, sandbox.S) string // returns expected working dir
		wantErr     bool
		errContains string
	}

	for _, tc := range []testcase{
		{
			name: "empty path defaults to current directory",
			path: "",
			setup: func(_ *testing.T, s sandbox.S) string {
				return s.RootDir()
			},
			wantErr: false,
		},
		{
			name: "dot path resolves to current directory",
			path: ".",
			setup: func(_ *testing.T, s sandbox.S) string {
				return s.RootDir()
			},
			wantErr: false,
		},
		{
			name: "relative path resolves correctly",
			path: "subdir",
			setup: func(t *testing.T, s sandbox.S) string {
				subdir := filepath.Join(s.RootDir(), "subdir")
				err := os.MkdirAll(subdir, 0755)
				assert.NoError(t, err)
				return subdir
			},
			wantErr: false,
		},
		{
			name: "nested relative path resolves correctly",
			path: "nested/deep",
			setup: func(t *testing.T, s sandbox.S) string {
				nested := filepath.Join(s.RootDir(), "nested", "deep")
				err := os.MkdirAll(nested, 0755)
				assert.NoError(t, err)
				return nested
			},
			wantErr: false,
		},
		{
			name: "project-relative absolute path resolves correctly",
			path: "/absolute",
			setup: func(t *testing.T, s sandbox.S) string {
				absDir := filepath.Join(s.RootDir(), "absolute")
				err := os.MkdirAll(absDir, 0755)
				assert.NoError(t, err)
				return absDir
			},
			wantErr: false,
		},
		{
			name: "project-relative absolute path /modules/s3-module resolves correctly",
			path: "/modules/s3-module",
			setup: func(t *testing.T, s sandbox.S) string {
				moduleDir := filepath.Join(s.RootDir(), "modules", "s3-module")
				err := os.MkdirAll(moduleDir, 0755)
				assert.NoError(t, err)
				return moduleDir
			},
			wantErr: false,
		},
		{
			name: "non-existent path returns error",
			path: "nonexistent",
			setup: func(_ *testing.T, _ sandbox.S) string {
				return "" // not used for error case
			},
			wantErr:     true,
			errContains: "path does not exist",
		},
		{
			name: "file instead of directory returns error",
			path: "somefile",
			setup: func(t *testing.T, s sandbox.S) string {
				filePath := filepath.Join(s.RootDir(), "somefile")
				err := os.WriteFile(filePath, []byte("content"), 0644)
				assert.NoError(t, err)
				return "" // not used for error case
			},
			wantErr:     true,
			errContains: "path is not a directory",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			expectedDir := tc.setup(t, s)

			path := tc.path

			workingDir := s.RootDir()

			// Create an engine for testing
			testEngine := newEngineForTest(t, s)

			cli := &mockCLI{
				workingDir: workingDir,
				engine:     testEngine,
				printers:   printer.Printers{},
				cliConfig:  cliconfig.Config{},
				stderr:     os.Stderr,
				stdin:      os.Stdin,
				stdout:     os.Stdout,
			}

			spec := &Spec{Path: path}

			ctx := context.Background()
			err := spec.Exec(ctx, cli)

			if tc.wantErr {
				assert.IsTrue(t, err != nil, "expected error but got nil")
				if tc.errContains != "" {
					assert.IsTrue(t, err != nil && strings.Contains(err.Error(), tc.errContains),
						"error should contain %q, got: %v", tc.errContains, err)
				}
				// For error cases, workingDir should not be set (path validation fails early)
				assert.EqualStrings(t, "", spec.workingDir, "workingDir should not be set on error")
			} else {
				// For success cases, path resolution should work even if Exec fails later (e.g., git operations)
				// The important thing is that workingDir was set correctly
				gotDir, err2 := filepath.Abs(spec.workingDir)
				assert.NoError(t, err2)
				wantDir, err2 := filepath.Abs(expectedDir)
				assert.NoError(t, err2)
				assert.EqualStrings(t, wantDir, gotDir,
					"workingDir mismatch: want %q, got %q", wantDir, gotDir)
				// Note: Exec may fail on git operations, but path resolution should succeed
				// We don't assert NoError here because git operations will fail without a real engine
			}
		})
	}
}

func TestPathResolutionWithRealFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)

	// Create a terraform module directory structure
	moduleDir := filepath.Join(s.RootDir(), "modules", "my-module")
	err := os.MkdirAll(moduleDir, 0755)
	assert.NoError(t, err)

	// Create a variable.tf file
	varFile := filepath.Join(moduleDir, "variable.tf")
	err = os.WriteFile(varFile, []byte(`
variable "name" {
  type        = string
  description = "The name"
  default     = "test"
}
`), 0644)
	assert.NoError(t, err)

	// Create an engine for testing
	testEngine := newEngineForTest(t, s)

	cli := &mockCLI{
		workingDir: s.RootDir(),
		engine:     testEngine,
		printers:   printer.Printers{},
		cliConfig:  cliconfig.Config{},
		stderr:     os.Stderr,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
	}

	spec := &Spec{
		Path: "/modules/my-module", // project-relative absolute path
	}

	ctx := context.Background()
	err = spec.Exec(ctx, cli)

	// The command will fail because we don't have a real engine/git setup,
	// but we can verify that the path was resolved correctly before that failure
	assert.IsTrue(t, err != nil, "expected error due to missing git setup")
	assert.IsTrue(t, spec.workingDir != "", "workingDir should be set")

	gotDir, err := filepath.Abs(spec.workingDir)
	assert.NoError(t, err)
	wantDir, err := filepath.Abs(moduleDir)
	assert.NoError(t, err)
	assert.EqualStrings(t, wantDir, gotDir,
		"workingDir should be set to module directory: want %q, got %q", wantDir, gotDir)
}

func TestPathResolutionWithChdir(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name          string
		cliWorkingDir string // Simulates -C flag value
		path          string
		setup         func(*testing.T, sandbox.S) (string, string) // returns (expectedDir, cliWorkingDir)
		wantErr       bool
		errContains   string
	}

	for _, tc := range []testcase{
		{
			name:          "chdir to modules/, relative path s3-module/",
			cliWorkingDir: "", // will be set in setup
			path:          "s3-module",
			setup: func(t *testing.T, s sandbox.S) (string, string) {
				modulesDir := filepath.Join(s.RootDir(), "modules")
				s3ModuleDir := filepath.Join(modulesDir, "s3-module")
				err := os.MkdirAll(s3ModuleDir, 0755)
				assert.NoError(t, err)
				return s3ModuleDir, modulesDir
			},
			wantErr: false,
		},
		{
			name:          "chdir to modules/s3-module, path is .",
			cliWorkingDir: "", // will be set in setup
			path:          ".",
			setup: func(t *testing.T, s sandbox.S) (string, string) {
				s3ModuleDir := filepath.Join(s.RootDir(), "modules", "s3-module")
				err := os.MkdirAll(s3ModuleDir, 0755)
				assert.NoError(t, err)
				return s3ModuleDir, s3ModuleDir
			},
			wantErr: false,
		},
		{
			name:          "chdir to modules/s3-module, empty path",
			cliWorkingDir: "", // will be set in setup
			path:          "",
			setup: func(t *testing.T, s sandbox.S) (string, string) {
				s3ModuleDir := filepath.Join(s.RootDir(), "modules", "s3-module")
				err := os.MkdirAll(s3ModuleDir, 0755)
				assert.NoError(t, err)
				return s3ModuleDir, s3ModuleDir
			},
			wantErr: false,
		},
		{
			name:          "chdir to modules/, relative path with parent directory",
			cliWorkingDir: "", // will be set in setup
			path:          "../other-module",
			setup: func(t *testing.T, s sandbox.S) (string, string) {
				modulesDir := filepath.Join(s.RootDir(), "modules")
				otherModuleDir := filepath.Join(s.RootDir(), "other-module")
				err := os.MkdirAll(modulesDir, 0755)
				assert.NoError(t, err)
				err = os.MkdirAll(otherModuleDir, 0755)
				assert.NoError(t, err)
				return otherModuleDir, modulesDir
			},
			wantErr: false,
		},
		{
			name:          "chdir to modules/, nested relative path",
			cliWorkingDir: "", // will be set in setup
			path:          "s3-module/subdir",
			setup: func(t *testing.T, s sandbox.S) (string, string) {
				modulesDir := filepath.Join(s.RootDir(), "modules")
				targetDir := filepath.Join(modulesDir, "s3-module", "subdir")
				err := os.MkdirAll(targetDir, 0755)
				assert.NoError(t, err)
				return targetDir, modulesDir
			},
			wantErr: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			expectedDir, cliWorkingDir := tc.setup(t, s)

			// Ensure cliWorkingDir is absolute
			cliWorkingDirAbs, err := filepath.Abs(cliWorkingDir)
			assert.NoError(t, err)

			// Create an engine for testing
			testEngine := newEngineForTest(t, s)

			cli := &mockCLI{
				workingDir: cliWorkingDirAbs,
				engine:     testEngine,
				printers:   printer.Printers{},
				cliConfig:  cliconfig.Config{},
				stderr:     os.Stderr,
				stdin:      os.Stdin,
				stdout:     os.Stdout,
			}

			spec := &Spec{Path: tc.path}

			ctx := context.Background()
			err = spec.Exec(ctx, cli)

			if tc.wantErr {
				assert.IsTrue(t, err != nil, "expected error but got nil")
				if tc.errContains != "" {
					assert.IsTrue(t, err != nil && strings.Contains(err.Error(), tc.errContains),
						"error should contain %q, got: %v", tc.errContains, err)
				}
				// For error cases, workingDir should not be set (path validation fails early)
				assert.EqualStrings(t, "", spec.workingDir, "workingDir should not be set on error")
			} else {
				// For success cases, path resolution should work even if Exec fails later (e.g., git operations)
				// The important thing is that workingDir was set correctly
				gotDir, err2 := filepath.Abs(spec.workingDir)
				assert.NoError(t, err2)
				wantDir, err2 := filepath.Abs(expectedDir)
				assert.NoError(t, err2)
				assert.EqualStrings(t, wantDir, gotDir,
					"workingDir mismatch: want %q, got %q (cli.WorkingDir was %q)", wantDir, gotDir, cliWorkingDirAbs)
				// Note: Exec may fail on git operations, but path resolution should succeed
				// We don't assert NoError here because git operations will fail without a real engine
			}
		})
	}
}

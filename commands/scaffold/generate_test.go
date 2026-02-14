// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package scaffold

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

type testCLI struct {
	workingDir string
	printers   printer.Printers
	engine     *engine.Engine

	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (c *testCLI) Version() string            { return "test" }
func (c *testCLI) Product() string            { return "terramate" }
func (c *testCLI) PrettyProduct() string      { return "Terramate" }
func (c *testCLI) WorkingDir() string         { return c.workingDir }
func (c *testCLI) Printers() printer.Printers { return c.printers }
func (c *testCLI) Stdout() io.Writer          { return c.stdout }
func (c *testCLI) Stderr() io.Writer          { return c.stderr }
func (c *testCLI) Stdin() io.Reader           { return bytes.NewBuffer(nil) }
func (c *testCLI) Config() cliconfig.Config   { return cliconfig.Config{} }
func (c *testCLI) Engine() *engine.Engine     { return c.engine }
func (c *testCLI) Reload(_ context.Context) error {
	return c.engine.ReloadConfig()
}

var _ commands.CLI = (*testCLI)(nil)

func TestRunGenerateExecutesGenerateCommand(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		`f:terramate.tm.hcl:terramate {}`,
		`s:stacks/app`,
		`f:stacks/app/stack.tm.hcl:stack {}`,
		`f:stacks/app/gen.tm.hcl:generate_file "hello.txt" { content = "hi" }`,
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	printers := printer.Printers{
		Stdout: printer.NewPrinter(stdout),
		Stderr: printer.NewPrinter(stderr),
	}

	eng, found, err := engine.Load(
		t.Context(),
		s.RootDir(),
		false,
		cliconfig.Config{},
		engine.HumanMode,
		printers,
		0,
	)
	assert.NoError(t, err)
	assert.IsTrue(t, found, "expected project to be found")

	cachedir := t.TempDir()
	b := di.NewBindings(context.Background())
	assert.NoError(t, di.Bind(b, resolve.NewAPI(cachedir)))
	assert.NoError(t, di.Bind(b, generate.NewAPI()))
	ctx := di.WithBindings(context.Background(), b)

	cli := &testCLI{
		workingDir: s.RootDir(),
		printers:   printers,
		engine:     eng,
		stdout:     stdout,
		stderr:     stderr,
	}

	spec := &Spec{
		engine:     eng,
		printers:   printers,
		workingDir: s.RootDir(),
	}

	err = spec.runGenerate(ctx, cli)
	assert.NoError(t, err)

	generatedPath := filepath.Join(s.RootDir(), "stacks", "app", "hello.txt")
	_, err = os.Stat(generatedPath)
	assert.NoError(t, err)
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package clone provides the clone command.
package clone

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/terramate-io/terramate/commands"
	gencmd "github.com/terramate-io/terramate/commands/generate"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/stack"
)

// Spec is the command specification for the clone command.
type Spec struct {
	SrcDir          string
	DstDir          string
	SkipChildStacks bool
	NoGenerate      bool

	workingDir string
	printers   printer.Printers
	engine     *engine.Engine
}

// Name returns the name of the command.
func (s *Spec) Name() string {
	return "clone"
}

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any {
	return commands.RequireEngine()
}

// Exec executes the clone command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.printers = cli.Printers()
	s.engine = cli.Engine()

	srcdir := s.SrcDir
	destdir := s.DstDir
	skipChildStacks := s.SkipChildStacks

	// Convert to absolute paths
	absSrcdir := filepath.Join(s.workingDir, srcdir)
	absDestdir := filepath.Join(s.workingDir, destdir)

	n, err := stack.Clone(s.engine.Config(), absDestdir, absSrcdir, skipChildStacks)
	if err != nil {
		return errors.E(err, "cloning %s to %s", srcdir, destdir)
	}

	s.printers.Stdout.Println(fmt.Sprintf("Cloned %d stack(s) from %s to %s with success", n, srcdir, destdir))

	if s.NoGenerate {
		return nil
	}

	s.printers.Stdout.Println("Generating code on the new cloned stack(s)")
	generate := gencmd.Spec{}
	return generate.Exec(ctx, cli)
}

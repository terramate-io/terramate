// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package clone provides the clone command.
package clone

import (
	"context"
	"fmt"
	"path/filepath"

	gencmd "github.com/terramate-io/terramate/commands/generate"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/stack"
)

// Spec is the command specification for the clone command.
type Spec struct {
	Engine          *engine.Engine
	WorkingDir      string
	SrcDir          string
	DstDir          string
	SkipChildStacks bool
	NoGenerate      bool

	Printers printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string {
	return "clone"
}

// Exec executes the clone command.
func (s *Spec) Exec(ctx context.Context) error {
	srcdir := s.SrcDir
	destdir := s.DstDir
	skipChildStacks := s.SkipChildStacks

	// Convert to absolute paths
	absSrcdir := filepath.Join(s.WorkingDir, srcdir)
	absDestdir := filepath.Join(s.WorkingDir, destdir)

	n, err := stack.Clone(s.Engine.Config(), absDestdir, absSrcdir, skipChildStacks)
	if err != nil {
		return errors.E(err, "cloning %s to %s", srcdir, destdir)
	}

	s.Printers.Stdout.Println(fmt.Sprintf("Cloned %d stack(s) from %s to %s with success", n, srcdir, destdir))

	if s.NoGenerate {
		return nil
	}

	s.Printers.Stdout.Println("Generating code on the new cloned stack(s)")
	generate := gencmd.Spec{
		Engine:     s.Engine,
		WorkingDir: s.WorkingDir,
		Printers:   s.Printers,
	}
	return generate.Exec(ctx)
}

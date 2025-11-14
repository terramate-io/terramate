// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"context"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"

	hhcl "github.com/terramate-io/hcl/v2"
)

// GetConfigValueSpec is the command specification for the experimental get-config-value command.
type GetConfigValueSpec struct {
	Vars    []string
	Globals map[string]string
	AsJSON  bool

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *GetConfigValueSpec) Name() string { return "experimental get-config-value" }

// Requirements returns the requirements of the command.
func (s *GetConfigValueSpec) Requirements(context.Context, commands.CLI) any {
	return commands.RequireEngine()
}

// Exec executes the experimental get-config-value command.
func (s *GetConfigValueSpec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	evalctx, err := s.engine.DetectEvalContext(s.workingDir, s.Globals)
	if err != nil {
		return err
	}
	for _, exprStr := range s.Vars {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			return errors.E(err, "unable to parse expression")
		}

		iteratorTraversal, diags := hhcl.AbsTraversalForExpr(expr)
		if diags.HasErrors() {
			return errors.E(diags, "expected a variable accessor")
		}

		varns := iteratorTraversal.RootName()
		if varns != "terramate" && varns != "global" {
			return errors.E("only terramate and global variables are supported")
		}

		val, err := evalctx.Eval(expr)
		if err != nil {
			return errors.E(err, "evaluating expression: %s", exprStr)
		}

		err = outputEvalResult(s.printers.Stdout, val, s.AsJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"context"

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"

	hhcl "github.com/terramate-io/hcl/v2"
)

// GetConfigValueSpec is the command specification for the experimental get-config-value command.
type GetConfigValueSpec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	Vars       []string
	Globals    map[string]string
	AsJSON     bool
}

// Name returns the name of the command.
func (s *GetConfigValueSpec) Name() string { return "experimental get-config-value" }

// Exec executes the experimental get-config-value command.
func (s *GetConfigValueSpec) Exec(_ context.Context) error {
	evalctx, err := s.Engine.DetectEvalContext(s.WorkingDir, s.Globals)
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

		err = outputEvalResult(s.Printers.Stdout, val, s.AsJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

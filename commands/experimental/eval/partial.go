// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"context"

	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
)

// PartialSpec is the command specification for the partial-eval command.
type PartialSpec struct {
	Globals map[string]string
	Exprs   []string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *PartialSpec) Name() string { return "experimental partial-eval" }

// Requirements returns the requirements of the command.
func (s *PartialSpec) Requirements(context.Context, commands.CLI) any {
	return commands.RequireEngine()
}

// Exec executes the partial-eval command.
func (s *PartialSpec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	evalctx, err := s.engine.DetectEvalContext(s.workingDir, s.Globals)
	if err != nil {
		return err
	}
	for _, exprStr := range s.Exprs {
		expr, err := ast.ParseExpression(exprStr, "<cmdline>")
		if err != nil {
			return errors.E(err, "unable to parse expression")
		}
		newexpr, _, err := evalctx.PartialEval(expr)
		if err != nil {
			return errors.E(err, "partial eval %q", exprStr)
		}
		s.printers.Stdout.Println(string(hclwrite.Format(ast.TokensForExpression(newexpr).Bytes())))
	}

	return nil
}

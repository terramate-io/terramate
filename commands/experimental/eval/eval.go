// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package eval provides the "experimental eval" and "experimental partial-eval" commands.
package eval

import (
	"context"

	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/zclconf/go-cty/cty"
)

// Spec is the command specification for the experimental eval command.
type Spec struct {
	Globals map[string]string
	AsJSON  bool
	Exprs   []string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "experimental eval" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the experimental eval command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
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
		val, err := evalctx.Eval(expr)
		if err != nil {
			return errors.E(err, "eval %q", exprStr)
		}
		err = outputEvalResult(s.printers.Stdout, val, s.AsJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputEvalResult(p *printer.Printer, val cty.Value, asJSON bool) error {
	var data []byte
	if asJSON {
		var err error
		data, err = ctyjson.Marshal(val, val.Type())
		if err != nil {
			return errors.E(err, "converting value %s to json", val.GoString())
		}
	} else {
		if val.Type() == cty.String {
			data = []byte(val.AsString())
		} else {
			tokens := ast.TokensForValue(val)
			data = []byte(hclwrite.Format(tokens.Bytes()))
		}
	}

	p.Println(string(data))
	return nil
}

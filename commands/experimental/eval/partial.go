package eval

import (
	"context"

	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
)

type PartialSpec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	Globals    map[string]string
	Exprs      []string
}

func (s *PartialSpec) Name() string { return "experimental partial-eval" }

func (s *PartialSpec) Exec(ctx context.Context) error {
	evalctx, err := s.Engine.DetectEvalContext(s.WorkingDir, s.Globals)
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
		s.Printers.Stdout.Println(string(hclwrite.Format(ast.TokensForExpression(newexpr).Bytes())))
	}

	return nil
}

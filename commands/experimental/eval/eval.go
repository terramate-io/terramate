package eval

import (
	"context"

	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/zclconf/go-cty/cty"
)

type Spec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	Globals    map[string]string
	AsJSON     bool

	Exprs []string
}

func (s *Spec) Name() string { return "experimental eval" }

func (s *Spec) Exec(ctx context.Context) error {
	evalctx, err := s.Engine.DetectEvalContext(s.WorkingDir, s.Globals)
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
		err = outputEvalResult(s.Printers.Stdout, val, s.AsJSON)
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

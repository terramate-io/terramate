package schema

import (
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals3"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
)

const (
	// ErrSchema indicates that the configuration has an invalid schema.
	ErrSchema errors.Kind = "config has an invalid schema"
)

// Assert represents evaluated assert block configuration.
type Assert struct {
	Assertion bool
	Warning   bool
	Message   string
	Range     hhcl.Range
}

// EvalAssert evaluates a given assert configuration and returns its
// evaluated form.
func EvalAssert(g *globals3.G, cfg hcl.AssertConfig) (Assert, error) {
	res := Assert{}
	errs := errors.L()

	assertion, err := evalBool(g, cfg.Assertion, "assert.assertion")
	if err != nil {
		errs.Append(err)
	} else {
		res.Assertion = assertion
		res.Range = cfg.Assertion.Range()
	}

	message, err := evalString(g, cfg.Message, "assert.message")
	if err != nil {
		errs.Append(err)
	} else {
		res.Message = message
	}

	if cfg.Warning != nil {
		warning, err := evalBool(g, cfg.Warning, "assert.warning")
		if err != nil {
			errs.Append(err)
		} else {
			res.Warning = warning
		}
	}

	if err := errs.AsError(); err != nil {
		return Assert{}, err
	}

	return res, nil
}

func evalBool(g *globals3.G, expr hhcl.Expression, name string) (bool, error) {
	if expr == nil {
		return false, errors.E(ErrSchema, "%s must be defined", name)
	}
	val, err := g.Eval(expr)
	if err != nil {
		return false, errors.E(err, "evaluating %s", name)
	}
	if val.Type() != cty.Bool {
		return false, errors.E(ErrSchema, "%s must be boolean, got %v", name, val.Type().FriendlyName())
	}
	return val.True(), nil
}

func evalString(g *globals3.G, expr hhcl.Expression, name string) (string, error) {
	if expr == nil {
		return "", errors.E(ErrSchema, "%s must be defined", name)
	}
	val, err := g.Eval(expr)
	if err != nil {
		return "", errors.E(err, "evaluating %s", name)
	}
	if val.Type() != cty.String {
		return "", errors.E(ErrSchema, "%s must be string, got %v", name, val.Type().FriendlyName())
	}
	return val.AsString(), nil
}

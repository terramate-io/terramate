package runtime

import (
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

type Resolver struct {
	terramate cty.Value
}

func NewResolver(root *config.Root, stack *config.Stack) *Resolver {
	runtime := root.Runtime()
	runtime.Merge(stack.RuntimeValues(root))
	return &Resolver{
		terramate: cty.ObjectVal(runtime),
	}
}

func (r *Resolver) Root() string { return "terramate" }

func (r *Resolver) Prevalue() cty.Value { return r.terramate }

func (r *Resolver) LoadStmts() (eval.Stmts, error) {
	return eval.Stmts{}, nil
}

func (r *Resolver) LookupRef(ref eval.Ref) (eval.Stmts, error) {
	return eval.Stmts{}, nil
}

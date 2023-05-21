package stack

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
)

// EvalCtx represents the evaluation context of a stack.
type EvalCtx struct {
	*eval.Context

	root *config.Root
}

// NewEvalCtx creates a new stack evaluation context.
func NewEvalCtx(root *config.Root, stack *config.Stack, globals *eval.Object) *EvalCtx {
	evalctx := eval.NewContext(stdlib.Functions(stack.HostDir(root)))
	evalwrapper := &EvalCtx{
		Context: evalctx,
		root:    root,
	}
	evalwrapper.SetMetadata(stack)
	evalwrapper.SetGlobals(globals)
	return evalwrapper
}

// SetGlobals sets the given globals on the stack evaluation context.
func (e *EvalCtx) SetGlobals(g *eval.Object) {
	e.SetNamespace("global", g.AsValueMap())
}

// SetMetadata sets the given metadata on the stack evaluation context.
func (e *EvalCtx) SetMetadata(st *config.Stack) {
	runtime := e.root.Runtime()
	runtime.Merge(st.RuntimeValues(e.root))
	e.SetNamespace("terramate", runtime)
}

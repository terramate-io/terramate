package stack

import (
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
)

// NewEvalCtx creates a new evaluation context for a stack
func NewEvalCtx(stackpath string, sm Metadata, globals Globals) (*eval.Context, error) {
	logger := log.With().
		Str("action", "stack.NewEvalCtx()").
		Str("path", stackpath).
		Logger()

	evalctx := eval.NewContext(stackpath)

	logger.Trace().Msg("Add stack metadata evaluation namespace.")

	err := evalctx.SetNamespace("terramate", MetaToCtyMap(sm))
	if err != nil {
		return nil, errors.E(sm, err, "setting terramate namespace on eval context")
	}

	logger.Trace().Msg("Add global evaluation namespace.")

	if err := evalctx.SetNamespace("global", globals.Attributes()); err != nil {
		return nil, errors.E(sm, err, "setting global namespace on eval context")
	}

	return evalctx, nil
}

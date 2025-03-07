package run

import (
	"context"

	"github.com/terramate-io/terramate/engine"
)

type Spec struct {
	Engine     *engine.Engine
	WorkingDir string
}

func (s *Spec) Name() string { return "run" }

func (s *Spec) Exec(ctx context.Context) error {
	return nil
}

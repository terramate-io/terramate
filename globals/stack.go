package globals

import (
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/stdlib"
)

// ForStack loads from the config tree all globals defined for a given stack.
func ForStack(root *config.Root, projmeta project.Metadata, stackmeta stack.Metadata) EvalReport {
	ctx := eval.NewContext(stdlib.Functions(stackmeta.HostPath()))
	ctx.SetNamespace("terramate", stack.MetadataToCtyValues(projmeta, stackmeta))
	return ForDir(root, stackmeta.Path(), ctx)
}

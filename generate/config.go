package generate

import "github.com/mineiros-io/terramate/stack"

// StackCfg represents code generation configuration for a stack.
type StackCfg struct {
	BackendCfgFilename string
	LocalsFilename     string
}

func LoadStackCfg(root string, stack stack.S) (StackCfg, error) {
	return StackCfg{}, nil
}

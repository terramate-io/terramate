package exportedtf

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/stack"
)

// Configs represents a map of configurations for exported
// Terraform code. These configurations are HCL with
// evaluated values on them.
type Configs map[string]Config

// Config represents a configuration for exported Terraform code.
// Is contains HCL parsed code with evaluated values on it.
type Config hclsyntax.Body

// String returns a string representation of the configuration
// that is guaranteed to be valid HCL or an empty string if the config
// itself is empty.
func (c Config) String() string {
	return ""
}

// Load loads from the file system all export_as_terraform for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading export_as_terraform and merging them appropriately.
//
// More specific definitions (closer or at the stack) have precedence over
// less specific ones (closer or at the root dir).
//
// Metadata and globals for the stack are used on the evaluation of the
// export_as_terramate blocks.
//
// The returned result only contains evaluated values.
//
// The rootdir MUST be an absolute path.
func Load(rootdir string, sm stack.Metadata, globals *terramate.Globals) (Configs, error) {
	return nil, nil
}

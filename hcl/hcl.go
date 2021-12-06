package hcl

import "github.com/madlambda/spells/errutil"

// Module represents a terraform module.
// Note that only the fields relevant for terrastack are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

type Terrastack struct {
	// RequiredVersion contains the terrastack version required by the stack.
	RequiredVersion string

	// After is a list of non-duplicated stack entries that must run after the
	// current stack runs.
	After []string

	// Before is a list of non-duplicated stack entries that must run before the
	// current stack runs.
	Before []string
}

// Parser is an interface for terrastack parsers.
type Parser interface {
	Parse(path string) (Terrastack, error)
}

// ModuleParser is an interface for parsing just the modules from HCL files.
type ModuleParser interface {
	ParseModules(path string) ([]Module, error)
}

const (
	ErrHCLSyntax         errutil.Error = "HCL syntax error"
	ErrNoTerrastackBlock errutil.Error = "no \"terrastack\" block found"
	ErrInvalidRunOrder   errutil.Error = "invalid execution order definition"
)

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}

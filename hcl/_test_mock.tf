// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "hcl" {
  content = <<-EOT
package hcl // import "github.com/terramate-io/terramate/hcl"

Package hcl provides parsing functionality for Terramate HCL configuration.
It also provides printing and formatting for Terramate configuration.

const ErrHCLSyntax errors.Kind = "HCL syntax error" ...
const ErrScriptNoLabels errors.Kind = "terramate schema error: (script): must provide at least one label" ...
const SharingIsCaringExperimentName = "outputs-sharing"
const StackBlockType = "stack"
func IsRootConfig(rootdir string) (bool, error)
func MatchAnyGlob(globs []glob.Glob, s string) bool
func PrintConfig(w io.Writer, cfg Config) error
func PrintImports(w io.Writer, imports []string) error
func ValueAsStringList(val cty.Value) ([]string, error)
type AssertConfig struct{ ... }
type ChangeDetectionConfig struct{ ... }
type CloudConfig struct{ ... }
type Command ast.Attribute
    func NewScriptCommand(attr ast.Attribute) *Command
type Commands ast.Attribute
    func NewScriptCommands(attr ast.Attribute) *Commands
type Config struct{ ... }
    func NewConfig(dir string) (Config, error)
    func ParseDir(root string, dir string, experiments ...string) (Config, error)
type Evaluator interface{ ... }
type GenFileBlock struct{ ... }
type GenHCLBlock struct{ ... }
type GenerateConfig struct{ ... }
type GenerateRootConfig struct{ ... }
type GitConfig struct{ ... }
    func NewGitConfig() *GitConfig
type Input struct{ ... }
type Inputs []Input
type ManifestConfig struct{ ... }
type ManifestDesc struct{ ... }
type OptionalCheck int
    const CheckIsUnset OptionalCheck = iota ...
    func ToOptionalCheck(v bool) OptionalCheck
type Output struct{ ... }
type Outputs []Output
type RawConfig struct{ ... }
    func NewCustomRawConfig(handlers map[string]mergeHandler) RawConfig
    func NewTopLevelRawConfig() RawConfig
type RootConfig struct{ ... }
type RunConfig struct{ ... }
    func NewRunConfig() *RunConfig
type RunEnv struct{ ... }
type Script struct{ ... }
type ScriptJob struct{ ... }
type SharingBackend struct{ ... }
type SharingBackendType int
    const TerraformSharingBackend SharingBackendType = iota + 1
type SharingBackends []SharingBackend
type Stack struct{ ... }
type StackFilterConfig struct{ ... }
type TargetsConfig struct{ ... }
type TerragruntChangeDetectionEnabledOption int
    const TerragruntAutoOption TerragruntChangeDetectionEnabledOption = iota ...
type TerragruntConfig struct{ ... }
type Terramate struct{ ... }
type TerramateParser struct{ ... }
    func NewStrictTerramateParser(rootdir string, dir string, experiments ...string) (*TerramateParser, error)
    func NewTerramateParser(rootdir string, dir string, experiments ...string) (*TerramateParser, error)
type VendorConfig struct{ ... }
EOT

  filename = "${path.module}/mock-hcl.ignore"
}

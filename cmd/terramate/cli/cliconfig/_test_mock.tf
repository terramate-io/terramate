// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cliconfig" {
  content = <<-EOT
package cliconfig // import "github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"

Package cliconfig implements the parser and load of Terramate CLI Configuration
files (.terramaterc and terramate.rc).

const ErrInvalidAttributeType errors.Kind = "attribute with invalid type" ...
const DirEnv = "HOME"
const Filename = ".terramaterc"
type Config struct{ ... }
    func Load() (cfg Config, err error)
    func LoadFrom(fname string) (Config, error)
EOT

  filename = "${path.module}/mock-cliconfig.ignore"
}

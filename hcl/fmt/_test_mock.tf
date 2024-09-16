// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "fmt" {
  content = <<-EOT
package fmt // import "github.com/terramate-io/terramate/hcl/fmt"

Package fmt contains functions for formatting hcl config.

const ErrHCLSyntax errors.Kind = "HCL syntax error"
const ErrReadFile errors.Kind = "failed to read file"
func Format(src, filename string) (string, error)
func FormatAttributes(attrs map[string]cty.Value) string
func FormatMultiline(src, filename string) (string, error)
type FormatResult struct{ ... }
    func FormatFiles(basedir string, files []string) ([]FormatResult, error)
    func FormatTree(dir string) ([]FormatResult, error)
EOT

  filename = "${path.module}/mock-fmt.ignore"
}

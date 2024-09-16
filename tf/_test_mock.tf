// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tf" {
  content = <<-EOT
package tf // import "github.com/terramate-io/terramate/tf"

Package tf provides parsing and abstractions specific to Terraform.

const ErrUnsupportedModSrc errors.Kind = "unsupported module source" ...
const ErrHCLSyntax errors.Kind = "HCL syntax error"
func IsStack(path string) (bool, error)
type Module struct{ ... }
    func ParseModules(path string) ([]Module, error)
type Source struct{ ... }
    func ParseSource(modsource string) (Source, error)
EOT

  filename = "${path.module}/mock-tf.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "manifest" {
  content = <<-EOT
package manifest // import "github.com/terramate-io/terramate/scaffold/manifest"

Package manifest provides types and functions for loading package manifests.

type Bundle struct{ ... }
type Component struct{ ... }
type Package struct{ ... }
    func LoadFile(path string) ([]*Package, error)
EOT

  filename = "${path.module}/mock-manifest.ignore"
}

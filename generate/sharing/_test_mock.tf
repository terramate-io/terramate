// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "sharing" {
  content = <<-EOT
package sharing // import "github.com/terramate-io/terramate/generate/sharing"

Package sharing implements the loading of sharing related blocks.

type File struct{ ... }
    func PrepareFile(root *config.Root, filename string, inputs config.Inputs, ...) (File, error)
EOT

  filename = "${path.module}/mock-sharing.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "vendordownload" {
  content = <<-EOT
package vendordownload // import "github.com/terramate-io/terramate/commands/experimental/vendordownload"

Package vendordownload provides the vendor download command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-vendordownload.ignore"
}

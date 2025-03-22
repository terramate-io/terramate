// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "preview" {
  content = <<-EOT
package preview // import "github.com/terramate-io/terramate/cloud/preview"

Package preview contains functionality for the preview feature in Terramate
Cloud.

const ErrInvalidStackStatus = errors.Kind("invalid stack status")
type Layer string
type StackStatus string
    const StackStatusAffected StackStatus = "affected" ...
    func DerivePreviewStatus(exitCode int) StackStatus
EOT

  filename = "${path.module}/mock-preview.ignore"
}

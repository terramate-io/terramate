// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "status" {
  content = <<-EOT
package status // import "github.com/terramate-io/terramate/cloud/api/status"

func ParseFilters(stackStatus, deploymentStatus, driftStatus string) (resources.StatusFilters, error)
EOT

  filename = "${path.module}/mock-status.ignore"
}

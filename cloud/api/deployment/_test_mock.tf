// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "deployment" {
  content = <<-EOT
package deployment // import "github.com/terramate-io/terramate/cloud/deployment"

Package deployment provides types and helpers for cloud deployments.

const ErrInvalidStatus errors.Kind = "invalid deployment status" ...
type FilterStatus Status
    const UnhealthyFilter FilterStatus = FilterStatus(^OK) ...
    func NewStatusFilter(str string) (FilterStatus, error)
type Status uint8
    const OK Status = 1 << iota ...
    func NewStatus(str string) Status
EOT

  filename = "${path.module}/mock-deployment.ignore"
}

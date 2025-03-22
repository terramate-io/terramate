// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "drift" {
  content = <<-EOT
package drift // import "github.com/terramate-io/terramate/cloud/drift"

Package drift provides types and helpers for cloud drifts.

const ErrInvalidStatus errors.Kind = "invalid drift status" ...
type FilterStatus Status
    const UnhealthyFilter FilterStatus = FilterStatus(^OK) ...
    func NewStatusFilter(str string) (FilterStatus, error)
type Status uint8
    const OK Status = 1 << iota ...
    func NewStatus(str string) Status
EOT

  filename = "${path.module}/mock-drift.ignore"
}

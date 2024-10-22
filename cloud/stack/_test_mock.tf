// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "stack" {
  content = <<-EOT
package stack // import "github.com/terramate-io/terramate/cloud/stack"

Package stack provides types and helpers for cloud stacks.

const ErrInvalidStatus errors.Kind = "invalid stack status" ...
const AnyTarget = ""
type FilterStatus Status
    const UnhealthyFilter FilterStatus = FilterStatus(Drifted | Failed) ...
    func NewStatusFilter(str string) (FilterStatus, error)
type Status uint8
    const OK Status = 1 << iota ...
    func NewStatus(str string) Status
EOT

  filename = "${path.module}/mock-stack.ignore"
}

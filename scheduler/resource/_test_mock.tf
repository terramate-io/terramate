// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "resource" {
  content = <<-EOT
package resource // import "github.com/terramate-io/terramate/scheduler/resource"

Package resource defines different concurrent access strategies for resources.

type Bounded struct{ ... }
    func NewBounded(n int) *Bounded
type R interface{ ... }
type Throttled struct{ ... }
    func NewThrottled(requestsPerSecond int64) *Throttled
EOT

  filename = "${path.module}/mock-resource.ignore"
}

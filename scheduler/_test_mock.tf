// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "scheduler" {
  content = <<-EOT
package scheduler // import "github.com/terramate-io/terramate/scheduler"

Package scheduler defines schedulers to execute functions on DAGs based on a
specific scheduling strategy.

type Func[V any] func(V) error
type Parallel[V any] struct{ ... }
    func NewParallel[V any](d *dag.DAG[V], reverse bool) *Parallel[V]
type S[V any] interface{ ... }
type Sequential[V any] struct{ ... }
    func NewSequential[V any](d *dag.DAG[V], reverse bool) *Sequential[V]
EOT

  filename = "${path.module}/mock-scheduler.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "dag" {
  content = <<-EOT
package dag // import "github.com/terramate-io/terramate/run/dag"

Package dag provides the Directed-Acyclic-Graph (DAG) primitives required
by Terramate. The nodes can be added by providing both the descendants and
ancestors of each node but only the descendant -> ancestors relationship is
kept.

const ErrDuplicateNode errors.Kind = "duplicate node" ...
type DAG[V any] struct{ ... }
    func New[V any]() *DAG[V]
    func Transform[D, S any](from *DAG[S], f func(id ID, v S) (D, error)) (*DAG[D], error)
type ID string
type Visited map[ID]struct{}
EOT

  filename = "${path.module}/mock-dag.ignore"
}

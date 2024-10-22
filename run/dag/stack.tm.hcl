// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package dag // import \"github.com/terramate-io/terramate/run/dag\""
  description = "package dag // import \"github.com/terramate-io/terramate/run/dag\"\n\nPackage dag provides the Directed-Acyclic-Graph (DAG) primitives required\nby Terramate. The nodes can be added by providing both the descendants and\nancestors of each node but only the descendant -> ancestors relationship is\nkept.\n\nconst ErrDuplicateNode errors.Kind = \"duplicate node\" ...\ntype DAG[V any] struct{ ... }\n    func New[V any]() *DAG[V]\n    func Transform[D, S any](from *DAG[S], f func(id ID, v S) (D, error)) (*DAG[D], error)\ntype ID string\ntype Visited map[ID]struct{}"
  tags        = ["dag", "golang", "run"]
  id          = "4afcc942-9533-4d27-86d5-fa24d0b099be"
}

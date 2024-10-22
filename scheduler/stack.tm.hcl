// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package scheduler // import \"github.com/terramate-io/terramate/scheduler\""
  description = "package scheduler // import \"github.com/terramate-io/terramate/scheduler\"\n\nPackage scheduler defines schedulers to execute functions on DAGs based on a\nspecific scheduling strategy.\n\ntype Func[V any] func(V) error\ntype Parallel[V any] struct{ ... }\n    func NewParallel[V any](d *dag.DAG[V], reverse bool) *Parallel[V]\ntype S[V any] interface{ ... }\ntype Sequential[V any] struct{ ... }\n    func NewSequential[V any](d *dag.DAG[V], reverse bool) *Sequential[V]"
  tags        = ["golang", "scheduler"]
  id          = "5215a993-ae93-4799-a113-1adb9fdbc69e"
}

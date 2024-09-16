// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package resource // import \"github.com/terramate-io/terramate/scheduler/resource\""
  description = "package resource // import \"github.com/terramate-io/terramate/scheduler/resource\"\n\nPackage resource defines different concurrent access strategies for resources.\n\ntype Bounded struct{ ... }\n    func NewBounded(n int) *Bounded\ntype R interface{ ... }\ntype Throttled struct{ ... }\n    func NewThrottled(requestsPerSecond int64) *Throttled"
  tags        = ["golang", "resource", "scheduler"]
  id          = "580bae4e-4ca4-422d-8529-4b06849ce989"
}

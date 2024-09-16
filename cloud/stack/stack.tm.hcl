// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package stack // import \"github.com/terramate-io/terramate/cloud/stack\""
  description = "package stack // import \"github.com/terramate-io/terramate/cloud/stack\"\n\nPackage stack provides types and helpers for cloud stacks.\n\nconst ErrInvalidStatus errors.Kind = \"invalid stack status\" ...\nconst AnyTarget = \"\"\ntype FilterStatus Status\n    const UnhealthyFilter FilterStatus = FilterStatus(Drifted | Failed) ...\n    func NewStatusFilter(str string) (FilterStatus, error)\ntype Status uint8\n    const OK Status = 1 << iota ...\n    func NewStatus(str string) Status"
  tags        = ["cloud", "golang", "stack"]
  id          = "6f8698d5-9e2b-47ea-b795-9a98b868fbe6"
}

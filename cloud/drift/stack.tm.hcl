// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package drift // import \"github.com/terramate-io/terramate/cloud/drift\""
  description = "package drift // import \"github.com/terramate-io/terramate/cloud/drift\"\n\nPackage drift provides types and helpers for cloud drifts.\n\nconst ErrInvalidStatus errors.Kind = \"invalid drift status\" ...\ntype FilterStatus Status\n    const UnhealthyFilter FilterStatus = FilterStatus(^OK) ...\n    func NewStatusFilter(str string) (FilterStatus, error)\ntype Status uint8\n    const OK Status = 1 << iota ...\n    func NewStatus(str string) Status"
  tags        = ["cloud", "drift", "golang"]
  id          = "e4f72263-bee0-4dee-82d2-28ad9dd8516f"
}

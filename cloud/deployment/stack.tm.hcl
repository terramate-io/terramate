// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package deployment // import \"github.com/terramate-io/terramate/cloud/deployment\""
  description = "package deployment // import \"github.com/terramate-io/terramate/cloud/deployment\"\n\nPackage deployment provides types and helpers for cloud deployments.\n\nconst ErrInvalidStatus errors.Kind = \"invalid deployment status\" ...\ntype FilterStatus Status\n    const UnhealthyFilter FilterStatus = FilterStatus(^OK) ...\n    func NewStatusFilter(str string) (FilterStatus, error)\ntype Status uint8\n    const OK Status = 1 << iota ...\n    func NewStatus(str string) Status"
  tags        = ["cloud", "deployment", "golang"]
  id          = "fcad074d-7f0e-491d-a72a-02939b4bb9be"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package preview // import \"github.com/terramate-io/terramate/cloud/preview\""
  description = "package preview // import \"github.com/terramate-io/terramate/cloud/preview\"\n\nPackage preview contains functionality for the preview feature in Terramate\nCloud.\n\nconst ErrInvalidStackStatus = errors.Kind(\"invalid stack status\")\ntype Layer string\ntype StackStatus string\n    const StackStatusAffected StackStatus = \"affected\" ...\n    func DerivePreviewStatus(exitCode int) StackStatus"
  tags        = ["cloud", "golang", "preview"]
  id          = "de7aef4e-bf44-440b-833e-aebc5e8d2606"
}

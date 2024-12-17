// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package ci // import \"github.com/terramate-io/terramate/ci\""
  description = "package ci // import \"github.com/terramate-io/terramate/ci\"\n\ntype PlatformType int\n    const PlatformLocal PlatformType = iota ...\n    func DetectPlatformFromEnv(repo *git.Repository) PlatformType"
  tags        = ["ci", "golang"]
  id          = "d359d542-f3be-4114-af77-24d51721011d"
}

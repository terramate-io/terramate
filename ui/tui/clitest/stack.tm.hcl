// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package clitest // import \"github.com/terramate-io/terramate/cmd/terramate/cli/clitest\""
  description = "package clitest // import \"github.com/terramate-io/terramate/cmd/terramate/cli/clitest\"\n\nPackage clitest provides constants and errors kind reused by the cli\nimplementation and the e2e test infrastructure.\n\nconst CloudDisablingMessage = \"disabling the cloud features\" ...\nconst ErrCloud errors.Kind = \"unprocessable cloud feature\" ..."
  tags        = ["cli", "clitest", "cmd", "golang", "terramate"]
  id          = "5b536408-2561-4011-93e9-da47aa9656cf"
}

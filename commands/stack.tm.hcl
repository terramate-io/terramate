// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package commands // import \"github.com/terramate-io/terramate/commands\""
  description = "package commands // import \"github.com/terramate-io/terramate/commands\"\n\nPackage commands define all Terramate commands. All commands must: - Be defined\nin its own package (eg.: ./commands/generate) - Define a `<cmd>.Spec` type\ndeclaring all variables that control the command behavior. - Implement the\ncommands.Executor interface. - Never abort, panic or exit the process.\n\ntype Executor interface{ ... }"
  tags        = ["commands", "golang"]
  id          = "cefaf477-a56f-40f8-b708-975b0b3a3fcf"
}

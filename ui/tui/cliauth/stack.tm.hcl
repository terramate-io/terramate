// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package auth // import \"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth\""
  description = "package auth // import \"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth\"\n\nPackage auth provides the helper functions for loading the Terramate Cloud\ncredentials.\n\nconst ErrIDPNeedConfirmation errors.Kind ...\nfunc CredentialFile(clicfg cliconfig.Config) string\nfunc GithubLogin(output out.O, tmcBaseURL string, clicfg cliconfig.Config) error\nfunc GoogleLogin(output out.O, clicfg cliconfig.Config) error\ntype APIKey struct{ ... }\ntype Credencial interface{ ... }\n    func ProbingPrecedence(output out.O, client *cloud.Client, clicfg cliconfig.Config) []Credencial"
  tags        = ["auth", "cli", "cmd", "golang", "terramate", "tmcloud"]
  id          = "a4b7b6ac-a1f3-4153-8a40-9a58b61a3741"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package runner // import \"github.com/terramate-io/terramate/e2etests/internal/runner\""
  description = "package runner // import \"github.com/terramate-io/terramate/e2etests/internal/runner\"\n\nPackage runner provides helpers for compiling and running the Terramate binary\nwith the intent of doing e2e tests. Additionally, it also provides functions for\nbuilding and installing dependency binaries.\n\nvar HelperPath string\nvar HelperPathAsHCL string\nvar TerraformTestPath string\nvar TerraformVersion string\nfunc AssertRun(t *testing.T, got RunResult)\nfunc AssertRunResult(t *testing.T, got RunResult, want RunExpected)\nfunc BuildTerramate(projectRoot, binDir string) (string, error)\nfunc BuildTestHelper(projectRoot, binDir string) (string, error)\nfunc InstallTerraform(preferredVersion string) (string, string, func(), error)\nfunc RemoveEnv(environ []string, names ...string) []string\nfunc Setup(projectRoot string) (err error)\nfunc Teardown()\ntype CLI struct{ ... }\n    func NewCLI(t *testing.T, chdir string, env ...string) CLI\n    func NewInteropCLI(t *testing.T, chdir string, env ...string) CLI\ntype Cmd struct{ ... }\ntype RunExpected struct{ ... }\ntype RunResult struct{ ... }"
  tags        = ["golang", "internal", "runner"]
  id          = "6d40054a-a8cf-440c-a22b-5f186fd90af2"
}

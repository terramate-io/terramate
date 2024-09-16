// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package stdlib // import \"github.com/terramate-io/terramate/stdlib\""
  description = "package stdlib // import \"github.com/terramate-io/terramate/stdlib\"\n\nPackage stdlib implements the Terramate language functions.\n\nconst ErrTomlDecode errors.Kind = \"failed to decode toml content\"\nconst TomlExperimentName = \"toml-functions\"\nfunc AbspathFunc(basedir string) function.Function\nfunc Functions(basedir string, experiments []string) map[string]function.Function\nfunc HCLDecode() function.Function\nfunc HCLEncode() function.Function\nfunc HCLExpressionFunc() function.Function\nfunc Name(name string) string\nfunc NoFS(basedir string, experiments []string) map[string]function.Function\nfunc Regex() function.Function\nfunc TernaryFunc() function.Function\nfunc TomlDecode() function.Function\nfunc TomlEncode() function.Function\nfunc TryFunc() function.Function\nfunc VendorFunc(basedir, vendordir project.Path, stream chan<- event.VendorRequest) function.Function\nfunc VersionMatch() function.Function"
  tags        = ["golang", "stdlib"]
  id          = "707e82ed-bed0-42e4-8f99-a6a215b3d4cf"
}

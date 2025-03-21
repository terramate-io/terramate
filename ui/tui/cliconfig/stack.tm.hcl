// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package cliconfig // import \"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig\""
  description = "package cliconfig // import \"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig\"\n\nPackage cliconfig implements the parser and load of Terramate CLI Configuration\nfiles (.terramaterc and terramate.rc).\n\nconst ErrInvalidAttributeType errors.Kind = \"attribute with invalid type\" ...\nconst DirEnv = \"HOME\"\nconst Filename = \".terramaterc\"\ntype Config struct{ ... }\n    func Load() (cfg Config, err error)\n    func LoadFrom(fname string) (Config, error)"
  tags        = ["cli", "cliconfig", "cmd", "golang", "terramate"]
  id          = "ffda9adf-be7b-47fd-9558-a311bfb8c44c"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package cli // import \"github.com/terramate-io/terramate/cmd/terramate/cli\""
  description = "package cli // import \"github.com/terramate-io/terramate/cmd/terramate/cli\"\n\nPackage cli provides cli specific functionality.\n\nconst ErrCurrentHeadIsOutOfDate errors.Kind = \"current HEAD is out-of-date with the remote base branch\" ...\nconst ErrLoginRequired errors.Kind ...\nconst ProvisionerTerraform = \"terraform\" ...\nconst ErrRunFailed errors.Kind = \"execution failed\" ...\nfunc Exec(version string, args []string, stdin io.Reader, stdout io.Writer, ...)\ntype UIMode int\n    const HumanMode UIMode = iota ..."
  tags        = ["cli", "cmd", "golang", "terramate"]
  id          = "695fcfdb-0421-4ea3-b066-3547b1036db0"
}

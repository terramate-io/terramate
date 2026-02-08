// Copyright 2026 Google LLC
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package extension // import \"github.com/terramate-io/terramate/plugin/extension\""
  description = "package extension // import \"github.com/terramate-io/terramate/plugin/extension\"\n\nPackage extension defines the in-process extension interfaces.\n\ntype BindingsSetupHandler func(c CLI, bindings *di.Bindings) error\ntype CLI interface{ ... }\ntype CLIInfo interface{ ... }\ntype CommandSelector func(ctx context.Context, c CLI, command string, flags any) (commands.Command, error)\ntype Extension interface{ ... }\ntype PostInitEngineHook func(ctx context.Context, c CLI) error"
  tags        = ["extension", "golang", "plugin"]
  id          = "99688592-aa9e-48ff-968a-18e7fa5298b5"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package trigger // import \"github.com/terramate-io/terramate/stack/trigger\""
  description = "package trigger // import \"github.com/terramate-io/terramate/stack/trigger\"\n\nPackage trigger provides functionality that help manipulate stacks triggers.\n\nconst ErrTrigger errors.Kind = \"trigger failed\" ...\nconst DefaultContext = \"stack\"\nfunc Create(root *config.Root, path project.Path, kind Kind, reason string) error\nfunc Dir(rootdir string) string\nfunc StackPath(triggerFile project.Path) (project.Path, bool)\ntype Info struct{ ... }\n    func Is(root *config.Root, filename project.Path) (info Info, stack project.Path, exists bool, err error)\n    func ParseFile(path string) (Info, error)\ntype Kind string\n    const Changed Kind = \"changed\" ..."
  tags        = ["golang", "stack", "trigger"]
  id          = "c0f5d20b-0765-4c23-9c52-2905a1621caa"
}

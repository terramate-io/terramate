// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package sandbox // import \"github.com/terramate-io/terramate/test/sandbox\""
  description = "package sandbox // import \"github.com/terramate-io/terramate/test/sandbox\"\n\nPackage sandbox provides an easy way to setup isolated terramate projects that\ncan be used on testing, acting like sandboxes.\n\nIt helps with:\n\n- git initialization/operations - Terraform module creation - Terramate stack\ncreation\n\ntype AssertTreeOption func(o *assertTreeOptions)\n    func WithStrictStackValidation() AssertTreeOption\ntype DirEntry struct{ ... }\ntype FileEntry struct{ ... }\ntype Git struct{ ... }\n    func NewGit(t testing.TB, repodir string) *Git\n    func NewGitWithConfig(t testing.TB, cfg GitConfig) *Git\ntype GitConfig struct{ ... }\ntype S struct{ ... }\n    func New(t testing.TB) S\n    func NewWithGitConfig(t testing.TB, cfg GitConfig) S\n    func NoGit(t testing.TB, createProject bool) S\ntype StackEntry struct{ ... }"
  tags        = ["golang", "sandbox", "test"]
  id          = "b88ff761-d470-4bab-b830-c7dcc54e93f4"
}

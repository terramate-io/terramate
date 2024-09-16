// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package stack // import \"github.com/terramate-io/terramate/stack\""
  description = "package stack // import \"github.com/terramate-io/terramate/stack\"\n\nPackage stack defines all functionality around stacks, like loading, listing all\nstacks, etc.\n\nconst ErrInvalidStackDir errors.Kind = \"invalid stack directory\" ...\nconst DefaultFilename = \"stack.tm.hcl\"\nconst ErrCloneDestDirExists errors.Kind = \"clone dest dir exists\"\nfunc Clone(root *config.Root, destdir, srcdir string, skipChildStacks bool) (int, error)\nfunc Create(root *config.Root, stack config.Stack, imports ...string) (err error)\nfunc UpdateStackID(root *config.Root, stackdir string) (string, error)\ntype Entry struct{ ... }\n    func List(root *config.Root, cfg *config.Tree) ([]Entry, error)\ntype EntrySlice []Entry\ntype EvalCtx struct{ ... }\n    func NewEvalCtx(root *config.Root, stack *config.Stack, globals *eval.Object) *EvalCtx\ntype Manager struct{ ... }\n    func NewGitAwareManager(root *config.Root, git *git.Git) *Manager\n    func NewManager(root *config.Root) *Manager\ntype RepoChecks struct{ ... }\ntype Report struct{ ... }"
  tags        = ["golang", "stack"]
  id          = "a8048c52-cb21-454e-abdd-3245fe1e6ac3"
}

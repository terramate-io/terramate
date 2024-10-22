// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package run // import \"github.com/terramate-io/terramate/run\""
  description = "package run // import \"github.com/terramate-io/terramate/run\"\n\nPackage run provides facilities to run commands inside Terramate context and\nordering.\n\nconst ErrLoadingGlobals errors.Kind = \"loading globals to evaluate terramate.config.run.env configuration\" ...\nconst ErrNotFound errors.Kind = \"executable file not found in $PATH\"\nfunc BuildDAG(d *dag.DAG[*config.Stack], root *config.Root, s *config.Stack, ...) error\nfunc BuildDAGFromStacks[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (*dag.DAG[E], string, error)\nfunc LookPath(file string, environ []string) (string, error)\nfunc Sort[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (string, error)\ntype EnvVars []string\n    func LoadEnv(root *config.Root, st *config.Stack) (EnvVars, error)"
  tags        = ["golang", "run"]
  id          = "aa1d4216-7901-4f5b-aeab-1d1ece0baa2b"
}

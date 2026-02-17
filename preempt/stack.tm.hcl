// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package preempt // import \"github.com/terramate-io/terramate/preempt\""
  description = "package preempt // import \"github.com/terramate-io/terramate/preempt\"\n\nPackage preempt implements cooperative scheduling for preemptable functions that\ncan await keys produced by other functions.\n\nfunc Await(ctx context.Context, key string) error\nfunc Run(ctx context.Context, fns iter.Seq[Preemptable]) error\ntype Preemptable func(ctx context.Context) (keys []string, err error)"
  tags        = ["golang", "preempt"]
  id          = "f169c2a4-f18b-4f54-800d-9b5b0e72fe8d"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package errors // import \"github.com/terramate-io/terramate/errors\""
  description = "package errors // import \"github.com/terramate-io/terramate/errors\"\n\nPackage errors implements the Terramate standard error type. It's heavily\ninfluenced by Rob Pike `errors` package in the Upspin project:\n\n    https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html\n\nfunc As(err error, target interface{}) bool\nfunc HasCode(err error, code Kind) bool\nfunc Is(err, target error) bool\nfunc IsAnyKind(err error, kinds ...Kind) bool\nfunc IsKind(err error, k Kind) bool\ntype DetailedError struct{ ... }\n    func D(format string, a ...any) *DetailedError\ntype Error struct{ ... }\n    func E(args ...interface{}) *Error\ntype ErrorDetails struct{ ... }\ntype Kind string\n    const ErrInternal Kind = \"terramate internal error\"\ntype List struct{ ... }\n    func L(errs ...error) *List"
  tags        = ["errors", "golang"]
  id          = "888fde98-5c68-4ac0-8e39-a5e0e6b926cd"
}

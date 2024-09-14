// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package errors // import \"github.com/terramate-io/terramate/test/errors\""
  description = "package errors // import \"github.com/terramate-io/terramate/test/errors\"\n\nPackage errors provides useful assert functions for handling errors on tests\n\nfunc Assert(t *testing.T, err, target error, args ...interface{})\nfunc AssertAsErrorsList(t *testing.T, err error)\nfunc AssertErrorList(t *testing.T, err error, targets []error)\nfunc AssertIsErrors(t *testing.T, err error, targets []error)\nfunc AssertIsKind(t *testing.T, err error, k errors.Kind)\nfunc AssertKind(t *testing.T, got, want error)"
  tags        = ["errors", "golang", "test"]
  id          = "1799cd5a-2c75-4fea-a381-18f31a7b0375"
}

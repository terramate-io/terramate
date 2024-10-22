// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package ls // import \"github.com/terramate-io/terramate/test/ls\""
  description = "package ls // import \"github.com/terramate-io/terramate/test/ls\"\n\nPackage ls provides test utilities used when testing the Terramate Language\nServer.\n\nfunc DefaultInitializeResult() lsp.InitializeResult\ntype Editor struct{ ... }\n    func NewEditor(t *testing.T, s sandbox.S, conn jsonrpc2.Conn) *Editor\ntype Fixture struct{ ... }\n    func Setup(t *testing.T, layout ...string) Fixture\n    func SetupNoRootConfig(t *testing.T, layout ...string) Fixture"
  tags        = ["golang", "ls", "test"]
  id          = "288082d0-0e81-4e03-9792-cfaa2055cef6"
}

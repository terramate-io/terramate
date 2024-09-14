// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package tmls // import \"github.com/terramate-io/terramate/ls\""
  description = "package tmls // import \"github.com/terramate-io/terramate/ls\"\n\nPackage tmls implements a Terramate Language Server (LSP).\n\nconst ErrUnrecognizedCommand errors.Kind = \"terramate-ls: unknown command\" ...\nconst MethodExecuteCommand = \"workspace/executeCommand\"\ntype Server struct{ ... }\n    func NewServer(conn jsonrpc2.Conn) *Server\n    func ServerWithLogger(conn jsonrpc2.Conn, l zerolog.Logger) *Server"
  tags        = ["golang", "ls"]
  id          = "adaa9d27-4d58-4fa9-a160-a8e92ee70db6"
}

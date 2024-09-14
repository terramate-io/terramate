// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "ls" {
  content = <<-EOT
package tmls // import "github.com/terramate-io/terramate/ls"

Package tmls implements a Terramate Language Server (LSP).

const ErrUnrecognizedCommand errors.Kind = "terramate-ls: unknown command" ...
const MethodExecuteCommand = "workspace/executeCommand"
type Server struct{ ... }
    func NewServer(conn jsonrpc2.Conn) *Server
    func ServerWithLogger(conn jsonrpc2.Conn, l zerolog.Logger) *Server
EOT

  filename = "${path.module}/mock-ls.ignore"
}

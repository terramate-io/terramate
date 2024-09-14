// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "ls" {
  content = <<-EOT
package ls // import "github.com/terramate-io/terramate/test/ls"

Package ls provides test utilities used when testing the Terramate Language
Server.

func DefaultInitializeResult() lsp.InitializeResult
type Editor struct{ ... }
    func NewEditor(t *testing.T, s sandbox.S, conn jsonrpc2.Conn) *Editor
type Fixture struct{ ... }
    func Setup(t *testing.T, layout ...string) Fixture
    func SetupNoRootConfig(t *testing.T, layout ...string) Fixture
EOT

  filename = "${path.module}/mock-ls.ignore"
}

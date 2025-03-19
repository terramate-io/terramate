// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "out" {
  content = <<-EOT
package out // import "github.com/terramate-io/terramate/cmd/terramate/cli/out"

Package out provides output functionality, including verboseness level and
normal/error messages support.

type O struct{ ... }
    func New(verboseness int, stdout, stderr io.Writer) O
EOT

  filename = "${path.module}/mock-out.ignore"
}

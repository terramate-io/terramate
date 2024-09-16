// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "printer" {
  content = <<-EOT
package printer // import "github.com/terramate-io/terramate/printer"

Package printer defines funtionality for "printing" text to an io.Writer e.g.
os.Stdout, os.Stderr etc. with a consistent style for errors, warnings,
information etc.

var Stderr = NewPrinter(os.Stderr) ...
type Printer struct{ ... }
    func NewPrinter(w io.Writer) *Printer
EOT

  filename = "${path.module}/mock-printer.ignore"
}

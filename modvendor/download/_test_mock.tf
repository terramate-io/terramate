// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "download" {
  content = <<-EOT
package download // import "github.com/terramate-io/terramate/modvendor/download"

Package download is responsible for downloading vendored modules.

const ErrAlreadyVendored errors.Kind = "module is already vendored" ...
func HandleVendorRequests(rootdir string, vendorRequests <-chan event.VendorRequest, ...) <-chan Report
func MergeVendorReports(reports <-chan Report) <-chan Report
type IgnoredVendor struct{ ... }
type ProgressEventStream event.Stream[event.VendorProgress]
    func NewEventStream() ProgressEventStream
type Report struct{ ... }
    func NewReport(vendordir project.Path) Report
    func Vendor(rootdir string, vendorDir project.Path, modsrc tf.Source, ...) Report
    func VendorAll(rootdir string, vendorDir project.Path, tfdir string, ...) Report
type Vendored struct{ ... }
EOT

  filename = "${path.module}/mock-download.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "report" {
  content = <<-EOT
package report // import "github.com/terramate-io/terramate/generate/report"

Package report provides a report of the code generation process.

type Dir struct{ ... }
type FailureResult struct{ ... }
type Report struct{ ... }
    func Merge(reportChan chan *Report) *Report
type Result struct{ ... }
EOT

  filename = "${path.module}/mock-report.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "test" {
  content = <<-EOT
package test // import "github.com/terramate-io/terramate/mapexpr/test"

Package test implements testcases and test helpers for dealing with map blocks.

type Testcase struct{ ... }
    func SchemaErrorTestcases() []Testcase
EOT

  filename = "${path.module}/mock-test.ignore"
}

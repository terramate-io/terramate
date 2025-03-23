// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "clitest" {
  content = <<-EOT
package clitest // import "github.com/terramate-io/terramate/cmd/terramate/cli/clitest"

Package clitest provides constants and errors kind reused by the cli
implementation and the e2e test infrastructure.

const CloudDisablingMessage = "disabling the cloud features" ...
const ErrCloud errors.Kind = "unprocessable cloud feature" ...
EOT

  filename = "${path.module}/mock-clitest.ignore"
}

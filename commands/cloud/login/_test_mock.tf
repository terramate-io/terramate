// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "login" {
  content = <<-EOT
package login // import "github.com/terramate-io/terramate/commands/cloud/login"

Package login provides login commands.

type GithubSpec struct{ ... }
type GoogleSpec struct{ ... }
type SSOSpec struct{ ... }
EOT

  filename = "${path.module}/mock-login.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "gitlab" {
  content = <<-EOT
package gitlab // import "github.com/terramate-io/terramate/cmd/terramate/cli/gitlab"

Package gitlab provides a SDK and helpers for the gitlab provider.

const ErrNotFound errors.Kind = "resource not found (HTTP Status: 404)"
type Client struct{ ... }
type MR struct{ ... }
type MRs []MR
type User struct{ ... }
EOT

  filename = "${path.module}/mock-gitlab.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "bitbucket" {
  content = <<-EOT
package bitbucket // import "github.com/terramate-io/terramate/cmd/terramate/cli/bitbucket"

Package bitbucket implements the client for Bitbucket Cloud.

type Actor struct{ ... }
type Branch struct{ ... }
type Client struct{ ... }
type Commit struct{ ... }
type PR struct{ ... }
type PRs []PR
type PullRequestResponse struct{ ... }
type Rendered struct{ ... }
type RenderedContent struct{ ... }
type Summary RenderedContent
type TargetBranch struct{ ... }
EOT

  filename = "${path.module}/mock-bitbucket.ignore"
}

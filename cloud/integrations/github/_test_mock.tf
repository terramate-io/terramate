// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "github" {
  content = <<-EOT
package github // import "github.com/terramate-io/terramate/cmd/terramate/cli/github"

Package github implements a client SDK for the Github API.

const ErrNotFound errors.Kind = "resource not found (HTTP Status: 404)" ...
const Domain = "github.com" ...
const ErrGithubEventPathEnvNotSet errors.Kind = `environment variable "GITHUB_EVENT_PATH" not set` ...
func GetEventPR() (*github.PullRequest, error)
func OIDCToken(ctx context.Context, cfg OIDCVars) (token string, err error)
type OAuthDeviceFlowContext struct{ ... }
    func OAuthDeviceFlowAuthStart(clientID string) (oauthCtx OAuthDeviceFlowContext, err error)
type OIDCVars struct{ ... }
EOT

  filename = "${path.module}/mock-github.ignore"
}

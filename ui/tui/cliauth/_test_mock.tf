// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cliauth" {
  content = <<-EOT
package auth // import "github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"

Package auth provides the helper functions for loading the Terramate Cloud
credentials.

const ErrIDPNeedConfirmation errors.Kind ...
func CredentialFile(clicfg cliconfig.Config) string
func GithubLogin(output out.O, tmcBaseURL string, clicfg cliconfig.Config) error
func GoogleLogin(output out.O, clicfg cliconfig.Config) error
type APIKey struct{ ... }
type Credencial interface{ ... }
    func ProbingPrecedence(output out.O, client *cloud.Client, clicfg cliconfig.Config) []Credencial
EOT

  filename = "${path.module}/mock-cliauth.ignore"
}

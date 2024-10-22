// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "git" {
  content = <<-EOT
package git // import "github.com/terramate-io/terramate/git"

Package git provides a wrapper for the git command line program. The helper
methods avoids porcelain commands as much as possible and return a parsed output
whenever possible.

Users of this package have access to the low-level Exec() function for the
methods not yet implemented.

const ErrInvalidGitURL errors.Kind = "invalid git remote URL"
func IsURL(u string) bool
func NewCmdError(cmd string, stdout, stderr []byte) error
func ParseURL(rawURL string) (u *url.URL, err error)
func RepoInfoFromURL(u *url.URL) (host string, owner string, name string, err error)
type CmdError struct{ ... }
type CommitMetadata struct{ ... }
type Config struct{ ... }
type Error string
    const ErrGitNotFound Error = "git program not found" ...
type Git struct{ ... }
    func WithConfig(cfg Config) (*Git, error)
type LogLine struct{ ... }
type Options struct{ ... }
type Ref struct{ ... }
type Remote struct{ ... }
type Repository struct{ ... }
    func NormalizeGitURI(raw string) (Repository, error)
EOT

  filename = "${path.module}/mock-git.ignore"
}

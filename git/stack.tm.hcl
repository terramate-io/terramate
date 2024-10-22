// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package git // import \"github.com/terramate-io/terramate/git\""
  description = "package git // import \"github.com/terramate-io/terramate/git\"\n\nPackage git provides a wrapper for the git command line program. The helper\nmethods avoids porcelain commands as much as possible and return a parsed output\nwhenever possible.\n\nUsers of this package have access to the low-level Exec() function for the\nmethods not yet implemented.\n\nconst ErrInvalidGitURL errors.Kind = \"invalid git remote URL\"\nfunc IsURL(u string) bool\nfunc NewCmdError(cmd string, stdout, stderr []byte) error\nfunc ParseURL(rawURL string) (u *url.URL, err error)\nfunc RepoInfoFromURL(u *url.URL) (host string, owner string, name string, err error)\ntype CmdError struct{ ... }\ntype CommitMetadata struct{ ... }\ntype Config struct{ ... }\ntype Error string\n    const ErrGitNotFound Error = \"git program not found\" ...\ntype Git struct{ ... }\n    func WithConfig(cfg Config) (*Git, error)\ntype LogLine struct{ ... }\ntype Options struct{ ... }\ntype Ref struct{ ... }\ntype Remote struct{ ... }\ntype Repository struct{ ... }\n    func NormalizeGitURI(raw string) (Repository, error)"
  tags        = ["git", "golang"]
  id          = "04c83acd-c138-4b51-bd77-d342b0209780"
}

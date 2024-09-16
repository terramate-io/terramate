// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "sandbox" {
  content = <<-EOT
package sandbox // import "github.com/terramate-io/terramate/test/sandbox"

Package sandbox provides an easy way to setup isolated terramate projects that
can be used on testing, acting like sandboxes.

It helps with:

- git initialization/operations - Terraform module creation - Terramate stack
creation

type AssertTreeOption func(o *assertTreeOptions)
    func WithStrictStackValidation() AssertTreeOption
type DirEntry struct{ ... }
type FileEntry struct{ ... }
type Git struct{ ... }
    func NewGit(t testing.TB, repodir string) *Git
    func NewGitWithConfig(t testing.TB, cfg GitConfig) *Git
type GitConfig struct{ ... }
type S struct{ ... }
    func New(t testing.TB) S
    func NewWithGitConfig(t testing.TB, cfg GitConfig) S
    func NoGit(t testing.TB, createProject bool) S
type StackEntry struct{ ... }
EOT

  filename = "${path.module}/mock-sandbox.ignore"
}

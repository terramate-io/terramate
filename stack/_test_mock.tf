// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "stack" {
  content = <<-EOT
package stack // import "github.com/terramate-io/terramate/stack"

Package stack defines all functionality around stacks, like loading, listing all
stacks, etc.

const ErrInvalidStackDir errors.Kind = "invalid stack directory" ...
const DefaultFilename = "stack.tm.hcl"
const ErrCloneDestDirExists errors.Kind = "clone dest dir exists"
func Clone(root *config.Root, destdir, srcdir string, skipChildStacks bool) (int, error)
func Create(root *config.Root, stack config.Stack, imports ...string) (err error)
func UpdateStackID(root *config.Root, stackdir string) (string, error)
type Entry struct{ ... }
    func List(root *config.Root, cfg *config.Tree) ([]Entry, error)
type EntrySlice []Entry
type EvalCtx struct{ ... }
    func NewEvalCtx(root *config.Root, stack *config.Stack, globals *eval.Object) *EvalCtx
type Manager struct{ ... }
    func NewGitAwareManager(root *config.Root, git *git.Git) *Manager
    func NewManager(root *config.Root) *Manager
type RepoChecks struct{ ... }
type Report struct{ ... }
EOT

  filename = "${path.module}/mock-stack.ignore"
}

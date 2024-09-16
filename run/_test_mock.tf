// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "run" {
  content = <<-EOT
package run // import "github.com/terramate-io/terramate/run"

Package run provides facilities to run commands inside Terramate context and
ordering.

const ErrLoadingGlobals errors.Kind = "loading globals to evaluate terramate.config.run.env configuration" ...
const ErrNotFound errors.Kind = "executable file not found in $PATH"
func BuildDAG(d *dag.DAG[*config.Stack], root *config.Root, s *config.Stack, ...) error
func BuildDAGFromStacks[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (*dag.DAG[E], string, error)
func LookPath(file string, environ []string) (string, error)
func Sort[S ~[]E, E any](root *config.Root, items S, getStack func(E) *config.Stack) (string, error)
type EnvVars []string
    func LoadEnv(root *config.Root, st *config.Stack) (EnvVars, error)
EOT

  filename = "${path.module}/mock-run.ignore"
}

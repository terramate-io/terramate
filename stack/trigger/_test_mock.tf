// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "trigger" {
  content = <<-EOT
package trigger // import "github.com/terramate-io/terramate/stack/trigger"

Package trigger provides functionality that help manipulate stacks triggers.

const ErrTrigger errors.Kind = "trigger failed" ...
const DefaultContext = "stack"
func Create(root *config.Root, path project.Path, kind Kind, reason string) error
func Dir(rootdir string) string
func StackPath(triggerFile project.Path) (project.Path, bool)
type Info struct{ ... }
    func Is(root *config.Root, filename project.Path) (info Info, stack project.Path, exists bool, err error)
    func ParseFile(path string) (Info, error)
type Kind string
    const Changed Kind = "changed" ...
EOT

  filename = "${path.module}/mock-trigger.ignore"
}

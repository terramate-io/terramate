// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "scaffold" {
  content = <<-EOT
package scaffold // import "github.com/terramate-io/terramate/commands/scaffold"

Package scaffold provides the scaffold command.

type DynamicForm struct{ ... }
    func NewDynamicForm(formFuncs ...func(*DynamicFormState) (*huh.Form, error)) (DynamicForm, error)
type DynamicFormState struct{ ... }
type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-scaffold.ignore"
}

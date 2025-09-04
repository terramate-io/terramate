// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "di" {
  content = <<-EOT
package di // import "github.com/terramate-io/terramate/di"

func Bind[Ifc, Impl any](b *Bindings, factory func(context.Context) (Impl, error)) error
func Get[Ifc any](ctx context.Context) (Ifc, error)
func InitAll(b *Bindings) error
func Override[Ifc, Impl any](b *Bindings, factory func(context.Context, Ifc) (Impl, error)) error
func Require[Ifc any](b *Bindings)
func Validate(b *Bindings) error
func WithBindings(ctx context.Context, b *Bindings) context.Context
type Bindings struct{ ... }
    func NewBindings(ctx context.Context) *Bindings
EOT

  filename = "${path.module}/mock-di.ignore"
}

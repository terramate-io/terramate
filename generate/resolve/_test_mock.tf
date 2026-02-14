// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "resolve" {
  content = <<-EOT
package resolve // import "github.com/terramate-io/terramate/generate/resolve"

Package resolve is responsible for resolving and fetching sources for package
items.

const ComponentsDir = "components" ...
func CombineSources(src, parentSrc string) string
func NewAPI(cachedir string) di.Factory[API]
type API interface{ ... }
type Kind int
    const Bundle Kind = iota ...
type Option func(API, *OptionValues)
    func WithParentSource(parentSrc string) Option
type OptionValues struct{ ... }
type Resolver struct{ ... }
EOT

  filename = "${path.module}/mock-resolve.ignore"
}

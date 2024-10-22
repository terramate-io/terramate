// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "generate" {
  content = <<-EOT
package generate // import "github.com/terramate-io/terramate/generate"

Package generate implements code generation. It includes all available code
generation strategies on Terramate and it also handles outdated code detection
and deletion.

const ErrLoadingGlobals errors.Kind = "loading globals" ...
func DetectOutdated(root *config.Root, target *config.Tree, vendorDir project.Path) ([]string, error)
func ListGenFiles(root *config.Root, dir string) ([]string, error)
type FailureResult struct{ ... }
type GenFile interface{ ... }
type LoadResult struct{ ... }
    func Load(root *config.Root, vendorDir project.Path) ([]LoadResult, error)
type Report struct{ ... }
    func Do(root *config.Root, dir project.Path, vendorDir project.Path, ...) Report
type Result struct{ ... }
EOT

  filename = "${path.module}/mock-generate.ignore"
}

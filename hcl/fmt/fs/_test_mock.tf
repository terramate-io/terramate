// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "fs" {
  content = <<-EOT
package fs // import "github.com/terramate-io/terramate/hcl/fmt/fs"

const ErrReadFile errors.Kind = "failed to read file"
type FormatResult struct{ ... }
    func FormatFiles(basedir string, files []string) ([]FormatResult, error)
    func FormatTree(root *config.Root, dir project.Path) ([]FormatResult, error)
EOT

  filename = "${path.module}/mock-fs.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "info" {
  content = <<-EOT
package info // import "github.com/terramate-io/terramate/test/hclutils/info"

Package info provides functions useful to create types like info.Range

func FixRange(rootdir string, old info.Range) info.Range
func FixRangesOnConfig(dir string, cfg hcl.Config)
func Range(fname string, start, end hhcl.Pos) info.Range
EOT

  filename = "${path.module}/mock-info.ignore"
}

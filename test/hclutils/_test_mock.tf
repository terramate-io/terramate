// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "hclutils" {
  content = <<-EOT
package hclutils // import "github.com/terramate-io/terramate/test/hclutils"

Package hclutils provides test utils related to hcl.

func End(line, column, char int) hhcl.Pos
func FixupFiledirOnErrorsFileRanges(dir string, errs []error)
func Mkrange(fname string, start, end hhcl.Pos) hhcl.Range
func Start(line, column, char int) hhcl.Pos
EOT

  filename = "${path.module}/mock-hclutils.ignore"
}

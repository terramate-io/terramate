// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "hclutils" {
  content = <<-EOT
package hclutils // import "github.com/terramate-io/terramate/test/hclwrite/hclutils"

Package hclutils provides useful functions to build HCL documents. It is usually
imported with . so building HCL documents can be done very fluently and yet in a
type safe manner.

func Assert(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Backend(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Block(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Bool(name string, val bool) hclwrite.BlockBuilder
func Command(args ...string) hclwrite.BlockBuilder
func Commands(args ...[]string) hclwrite.BlockBuilder
func Config(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Content(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Default(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Doc(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Env(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func EvalExpr(t *testing.T, name string, expr string) hclwrite.BlockBuilder
func Experiments(names ...string) hclwrite.BlockBuilder
func Expr(name string, expr string) hclwrite.BlockBuilder
func GenerateFile(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func GenerateHCL(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Globals(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Import(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Input(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Labels(labels ...string) hclwrite.BlockBuilder
func Lets(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Locals(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Manifest(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Map(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Module(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Number(name string, val int64) hclwrite.BlockBuilder
func Output(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func ProjectPaths(paths ...string) hclwrite.BlockBuilder
func RepositoryPaths(paths ...string) hclwrite.BlockBuilder
func Run(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Script(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Stack(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func StackFilter(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Str(name string, val string) hclwrite.BlockBuilder
func Terraform(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Terramate(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func TmDynamic(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Trigger(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Value(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Variable(builders ...hclwrite.BlockBuilder) *hclwrite.Block
func Vendor(builders ...hclwrite.BlockBuilder) *hclwrite.Block
EOT

  filename = "${path.module}/mock-hclutils.ignore"
}

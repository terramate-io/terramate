// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "genfile" {
  content = <<-EOT
package genfile // import "github.com/terramate-io/terramate/generate/genfile"

Package genfile implements generate_file code generation.

const ErrInvalidContentType errors.Kind = "invalid content type" ...
const StackContext = "stack" ...
type File struct{ ... }
    func Eval(block hcl.GenFileBlock, cfg *config.Tree, evalctx *eval.Context) (file File, skip bool, err error)
    func Load(root *config.Root, st *config.Stack, parentctx *eval.Context, ...) ([]File, error)
EOT

  filename = "${path.module}/mock-genfile.ignore"
}

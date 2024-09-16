// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "genhcl" {
  content = <<-EOT
package genhcl // import "github.com/terramate-io/terramate/generate/genhcl"

Package genhcl implements generate_hcl code generation.

const HeaderMagic = "TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT" ...
const ErrParsing errors.Kind = "parsing generate_hcl block" ...
func DefaultHeader() string
func Header(comment CommentStyle) string
type CommentStyle int
    const SlashComment CommentStyle = iota ...
    func CommentStyleFromConfig(tree *config.Tree) CommentStyle
type HCL struct{ ... }
    func Load(root *config.Root, st *config.Stack, evalctx *eval.Context, ...) ([]HCL, error)
EOT

  filename = "${path.module}/mock-genhcl.ignore"
}

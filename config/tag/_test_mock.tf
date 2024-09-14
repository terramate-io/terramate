// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tag" {
  content = <<-EOT
package tag // import "github.com/terramate-io/terramate/config/tag"

Package tag provides helpers for dealing with Terramate tags.

const ErrInvalidTag errors.Kind = "invalid tag"
func Validate(tag string) error
EOT

  filename = "${path.module}/mock-tag.ignore"
}

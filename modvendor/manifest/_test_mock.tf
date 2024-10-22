// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "manifest" {
  content = <<-EOT
package manifest // import "github.com/terramate-io/terramate/modvendor/manifest"

Package manifest implements vendor manifest parsing.

func LoadFileMatcher(rootdir string) (gitignore.Matcher, error)
EOT

  filename = "${path.module}/mock-manifest.ignore"
}

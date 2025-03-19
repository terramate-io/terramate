// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "script" {
  content = <<-EOT
package script // import "github.com/terramate-io/terramate/commands/script"

Package script provides the script command.

type InfoEntry struct{ ... }
type Matcher struct{ ... }
    func NewMatcher(labels []string) *Matcher
EOT

  filename = "${path.module}/mock-script.ignore"
}

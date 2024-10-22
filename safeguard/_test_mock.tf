// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "safeguard" {
  content = <<-EOT
package safeguard // import "github.com/terramate-io/terramate/safeguard"

Package safeguard provides types and methods for dealing with safeguards
keywords.

type Keyword string
    const All Keyword = "all" ...
type Keywords []Keyword
    func FromStrings(strs []string) (Keywords, error)
EOT

  filename = "${path.module}/mock-safeguard.ignore"
}

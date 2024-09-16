// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "filter" {
  content = <<-EOT
package filter // import "github.com/terramate-io/terramate/config/filter"

Package filter provides helpers for filtering objects.

func MatchTags(filter TagClause, tags []string) bool
func MatchTagsFrom(filters []string, tags []string) (bool, error)
type Operation int
    const EQ Operation = iota + 1 ...
type TagClause struct{ ... }
    func ParseTagClauses(filters ...string) (TagClause, bool, error)
EOT

  filename = "${path.module}/mock-filter.ignore"
}

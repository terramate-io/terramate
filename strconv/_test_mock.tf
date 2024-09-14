// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "strconv" {
  content = <<-EOT
package strconv // import "github.com/terramate-io/terramate/strconv"

Package strconv provides helper functions for the Go standard strconv package.

func Atoi64(a string) (int64, error)
func Itoa64(i int64) string
EOT

  filename = "${path.module}/mock-strconv.ignore"
}

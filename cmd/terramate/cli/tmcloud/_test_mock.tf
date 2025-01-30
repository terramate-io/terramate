// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tmcloud" {
  content = <<-EOT
package tmcloud // import "github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud"

Package tmcloud provides helper functions for interacting with the Terramate
Cloud API from the CLI.

func BaseURL() string
EOT

  filename = "${path.module}/mock-tmcloud.ignore"
}

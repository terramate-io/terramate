// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "versions" {
  content = <<-EOT
package versions // import "github.com/terramate-io/terramate/versions"

Package versions provide helper functions for version constraint matching.

const ErrCheck errors.Kind = "version check error"
func Check(version string, constraint string, allowPrereleases bool) error
func Match(version, constraint string, allowPrereleases bool) (bool, error)
EOT

  filename = "${path.module}/mock-versions.ignore"
}

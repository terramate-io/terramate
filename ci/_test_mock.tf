// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "ci" {
  content = <<-EOT
package ci // import "github.com/terramate-io/terramate/ci"

type PlatformType int
    const PlatformLocal PlatformType = iota ...
    func DetectPlatformFromEnv(repo *git.Repository) PlatformType
EOT

  filename = "${path.module}/mock-ci.ignore"
}

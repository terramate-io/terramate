// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "errlog" {
  content = <<-EOT
package errlog // import "github.com/terramate-io/terramate/errors/errlog"

Package errlog provides functions to log Terramate errors nicely and in a
consistent manner.

func Warn(logger zerolog.Logger, err error, args ...any)
EOT

  filename = "${path.module}/mock-errlog.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "os" {
  content = <<-EOT
package os // import "github.com/terramate-io/terramate/os"

type Path string
    func HostPath(wd Path, p string) Path
    func NewHostPath(p string) Path
type Paths []Path
EOT

  filename = "${path.module}/mock-os.ignore"
}

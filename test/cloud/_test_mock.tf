// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cloud" {
  content = <<-EOT
package cloud // import "github.com/terramate-io/terramate/test/cloud"

Package cloud provides testing helpers for the TMC cloud.

func PutStack(t *testing.T, addr string, orgUUID cloud.UUID, st cloud.StackObject)
EOT

  filename = "${path.module}/mock-cloud.ignore"
}

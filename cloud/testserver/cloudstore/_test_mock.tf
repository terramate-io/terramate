// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cloudstore" {
  content = <<-EOT
package cloudstore // import "github.com/terramate-io/terramate/cloud/testserver/cloudstore"

Package cloudstore provides the in-memory store used by the fake Terramate Cloud
server.

const ErrAlreadyExists errors.Kind = "record already exists" ...
type Data struct{ ... }
    func LoadDatastore(fpath string) (*Data, error)
type Deployment struct{ ... }
type DeploymentState struct{ ... }
type Drift struct{ ... }
type Member struct{ ... }
type Org struct{ ... }
type Preview struct{ ... }
type Stack struct{ ... }
type StackPreview struct{ ... }
type StackState struct{ ... }
    func NewState() StackState
EOT

  filename = "${path.module}/mock-cloudstore.ignore"
}

// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cloudsync" {
  content = <<-EOT
package cloudsync // import "github.com/terramate-io/terramate/cloudsync"

Package cloudsync provides helper functions for cloud sync operations.

const ProvisionerTerraform = "terraform" ...
func AfterRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, ...)
func BeforeRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState)
func CreateCloudDeployment(e *engine.Engine, wd string, deployRuns []engine.StackCloudRun, ...) error
func CreateCloudPreview(e *engine.Engine, gitfilter engine.GitFilter, runs []engine.StackCloudRun, ...) map[string]string
func DetectCloudMetadata(e *engine.Engine, state *CloudRunState)
func Logs(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, ...)
type CloudRunState struct{ ... }
EOT

  filename = "${path.module}/mock-cloudsync.ignore"
}

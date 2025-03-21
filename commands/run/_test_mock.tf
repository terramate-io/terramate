// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "run" {
  content = <<-EOT
package run // import "github.com/terramate-io/terramate/commands/run"

Package run provides the run command.

const ProvisionerTerraform = "terraform" ...
const ErrConflictOptions errors.Kind = "conflicting arguments" ...
func CheckOutdatedGeneratedCode(e *engine.Engine, sf Safeguards, wd string) error
func CloudSyncAfter(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, ...)
func CloudSyncBefore(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState)
func CloudSyncLogs(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, ...)
func CreateCloudDeployment(e *engine.Engine, wd string, deployRuns []engine.StackCloudRun, ...) error
func CreateCloudPreview(e *engine.Engine, gitfilter engine.GitFilter, runs []engine.StackCloudRun, ...) map[string]string
func DetectCloudMetadata(e *engine.Engine, state *CloudRunState)
func GitFileSafeguards(e *engine.Engine, shouldError bool, sf Safeguards) error
func GitSafeguardDefaultBranchIsReachable(engine *engine.Engine, safeguards Safeguards) error
func SelectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string)
type CloudRunState struct{ ... }
type Safeguards struct{ ... }
type Spec struct{ ... }
type StatusFilters struct{ ... }
EOT

  filename = "${path.module}/mock-run.ignore"
}

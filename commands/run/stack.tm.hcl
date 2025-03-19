// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package run // import \"github.com/terramate-io/terramate/commands/run\""
  description = "package run // import \"github.com/terramate-io/terramate/commands/run\"\n\nPackage run provides the run command.\n\nconst ProvisionerTerraform = \"terraform\" ...\nconst ErrConflictOptions errors.Kind = \"conflicting arguments\" ...\nfunc CheckOutdatedGeneratedCode(e *engine.Engine, sf Safeguards, wd string) error\nfunc CloudSyncAfter(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, ...)\nfunc CloudSyncBefore(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState)\nfunc CloudSyncLogs(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, ...)\nfunc CreateCloudDeployment(e *engine.Engine, wd string, deployRuns []engine.StackCloudRun, ...) error\nfunc CreateCloudPreview(e *engine.Engine, gitfilter engine.GitFilter, runs []engine.StackCloudRun, ...) map[string]string\nfunc DetectCloudMetadata(e *engine.Engine, state *CloudRunState)\nfunc GitFileSafeguards(e *engine.Engine, shouldError bool, sf Safeguards) error\nfunc GitSafeguardDefaultBranchIsReachable(engine *engine.Engine, safeguards Safeguards) error\nfunc SelectPlanFile(terraformPlan, tofuPlan string) (planfile, provisioner string)\ntype CloudRunState struct{ ... }\ntype Safeguards struct{ ... }\ntype Spec struct{ ... }\ntype StatusFilters struct{ ... }"
  tags        = ["commands", "golang", "run"]
  id          = "e5fdd03a-248b-4a54-82d5-79e8b9cf04bc"
}

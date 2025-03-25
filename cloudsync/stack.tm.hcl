// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package cloudsync // import \"github.com/terramate-io/terramate/cloudsync\""
  description = "package cloudsync // import \"github.com/terramate-io/terramate/cloudsync\"\n\nPackage cloudsync provides helper functions for cloud sync operations.\n\nconst ProvisionerTerraform = \"terraform\" ...\nfunc AfterRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState, ...)\nfunc BeforeRun(e *engine.Engine, run engine.StackCloudRun, state *CloudRunState)\nfunc CreateCloudDeployment(e *engine.Engine, wd string, deployRuns []engine.StackCloudRun, ...) error\nfunc CreateCloudPreview(e *engine.Engine, gitfilter engine.GitFilter, runs []engine.StackCloudRun, ...) map[string]string\nfunc DetectCloudMetadata(e *engine.Engine, state *CloudRunState)\nfunc Logs(logger *zerolog.Logger, e *engine.Engine, run engine.StackRun, ...)\ntype CloudRunState struct{ ... }"
  tags        = ["cloudsync", "golang"]
  id          = "302d5bb6-c900-4c71-81c9-4a9599a60707"
}

// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package cloudstore // import \"github.com/terramate-io/terramate/cloud/testserver/cloudstore\""
  description = "package cloudstore // import \"github.com/terramate-io/terramate/cloud/testserver/cloudstore\"\n\nPackage cloudstore provides the in-memory store used by the fake Terramate Cloud\nserver.\n\nconst ErrAlreadyExists errors.Kind = \"record already exists\" ...\ntype Data struct{ ... }\n    func LoadDatastore(fpath string) (*Data, error)\ntype Deployment struct{ ... }\ntype DeploymentState struct{ ... }\ntype Drift struct{ ... }\ntype Member struct{ ... }\ntype Org struct{ ... }\ntype Preview struct{ ... }\ntype Stack struct{ ... }\ntype StackPreview struct{ ... }\ntype StackState struct{ ... }\n    func NewState() StackState"
  tags        = ["cloud", "cloudstore", "golang", "testserver"]
  id          = "87319590-2a04-4eea-a900-23edab55f7a9"
}

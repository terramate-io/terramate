// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

globals "testserver" "stacks" "sample" {
  path             = "/sample/path"
  meta_name        = "sample"
  meta_description = "sample description"
  meta_id          = "deadbeef-dead-dead-dead-deaddeafbeef"
  meta_tags = [
    "sample-tag",
  ]
  repository     = "github.com/terramate-io/terramate"
  default_branch = "main"

  state = {
    status            = "ok"
    deployment_status = "ok"
    drift_status      = "ok"
    created_at        = "2023-11-03T00:19:42Z"
    updated_at        = "2023-11-03T00:19:42Z"
    seen_at           = "2023-11-03T00:19:42Z"
  }

  deployments = [
    {
      uuid   = "deadbeef-dead-dead-dead-deaddeafbeef"
      path   = "/"
      cmd    = "terraform apply"
      status = "ok"
    }
  ]
  drifts = [
    {
      uuid        = "deadbeef-dead-dead-dead-deaddeafbeef"
      status      = "ok"
      cmd         = ["terraform", "plan", "-detailed-exitcode"]
      started_at  = "2023-11-03T00:20:42Z"
      finished_at = "2023-11-03T00:21:42Z"
    }
  ]
}

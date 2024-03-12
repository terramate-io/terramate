// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

script "test" {
  description = "Run Terramate tests"
  job {
    command = global.cmd.test.command
  }
}

script "test" "preview" {
  description = "Create a Terramate Cloud Preview"
  job {
    command = tm_concat(global.cmd.test.command, [{
      cloud_sync_preview             = true,
      cloud_sync_terraform_plan_file = "test.plan"
    }])
  }
}

script "test" "deploy" {
  description = "Create a Terramate Cloud Deployment"
  job {
    command = tm_concat(global.cmd.test.command, [{
      cloud_sync_deployment = true,
    }])
  }
}

# Command variables

# Defines the "tm script run -- test" command.
globals "cmd" "test" {
  command = ["go", "test", "-race", "-count=1"]
}


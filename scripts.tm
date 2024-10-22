// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

globals {
  planfile = "out.tfplan"
}

script "test" {
  name = "Terramate tests"

  job {
    commands = [
      ["bash", "-c", "terraform init -lock=false >/dev/null"],
      ["bash", "-c", "terraform plan -out=${global.planfile} >/dev/null"]
    ]
  }

  job {
    command = [
    "go", "test", "-race", "-count=1", "-timeout", "30m"]
  }
}

script "preview" {
  name = "Preview Terramate tests"

  job {
    commands = [
      ["bash", "-c", "terraform init -lock=false >/dev/null"],
      ["bash", "-c", "terraform plan -out=${global.planfile} >/dev/null"]
    ]
  }

  job {
    command = [
      "go", "test", "-race", "-count=1", "-timeout", "30m", {
        sync_preview        = true,
        terraform_plan_file = global.planfile,
      }
    ]
  }
}

script "deploy" {
  name = "Deploy Terramate tests"

  job {
    commands = [
      ["bash", "-c", "terraform init -lock=false >/dev/null"],
      ["bash", "-c", "terraform plan -out=${global.planfile} >/dev/null"]
    ]
  }

  job {
    command = [
      "go", "test", "-race", "-count=1", "-timeout", "30m", {
        sync_deployment     = true,
        terraform_plan_file = global.planfile,
      }
    ]
  }
}

// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

terramate {
  config {
    experiments = ["toml-functions", "scripts", "targets"]
    run {
      env {
        PATH = "${terramate.root.path.fs.absolute}/bin${global.PS}${env.PATH}"
        NO_COLOR = "1" # needed for our printer tests
      }
    }

    git {
      check_untracked   = false
      check_uncommitted = false
      check_remote      = false
    }

    cloud {
      location     = "eu"
      organization = "terramate-tests"

      targets {
        enabled = true
      }
    }
  }
}

globals {
  # TODO(i4k): very brittle but works for now.
  PS = tm_fileexists("/etc/hosts") ? ":" : ";"
}


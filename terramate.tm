// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

# This file is here just to e2etest on Windows if the `terramate run` respects
# the `terramate.config.run.env.PATH` environment variable.
# This behavior is not tested in Go because it requires a lot of "unsafe"
# non-portable code.
# It's used by the `make test` implemented for Windows at ./makefiles/windows.mk

terramate {
  config {
    run {
      env {
        PATH = "${terramate.root.path.fs.absolute}/bin${global.PS}${env.PATH}"
      }
    }

    git {
      check_untracked   = false
      check_uncommitted = false
      check_remote      = false
    }
  }
}

globals {
  # TODO(i4k): very brittle but works for now.
  PS = tm_fileexists("/etc/hosts") ? ":" : ";"
}

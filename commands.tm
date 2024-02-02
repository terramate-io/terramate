// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

# Defines the "tm script run -- test all" command.
globals "cmd" "test" "all" {
  root_commands = [
    ["make", "mod/check"],
    ["make", "lint"],
    ["make", "license/check"],
    ["terramate", "generate"],
  ]
  commands = tm_concat(
    global.is_root ? global.cmd.test.all.root_commands : [],
    [global.cmd.test.command],
  )
}

script "test" "all" {
  description = "Run all Terramate checks and tests"
  job {
    commands = global.cmd.test.all.commands
  }
}

script "test" {
  description = "Run Terramate tests"
  job {
    command = global.cmd.test.command
  }
}

script "create" "stack" {
  description = <<-EOF
    Creates a new stack.

    Usage: 
      TAGS=golang,mytag STACK_PATH=./stackdir terramate script run -- create stack

  EOF
  job {
    # hack until we dont support context=root
    command = (global.is_root ?
      ["terramate", "create", "--tags", env.TAGS, env.STACK_PATH] :
    ["true"])
  }
}

# Command variables

globals {
  is_root = terramate.stack.path.absolute == "/"
}

# Defines the "tm script run -- test" command.
globals "cmd" "test" {
  command = ["go", "test", "-race", "-count=1"]
}

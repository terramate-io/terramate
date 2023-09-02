---
title: terramate run - Command
description: With the terramate run command you can execute any command in a single or a list of stacks.

prev:
  text: 'Run Order'
  link: '/cmdline/run-order'

next:
  text: 'Trigger'
  link: '/cmdline/trigger'
---

# Run

The `run` command executes **any command** in a single or a range of stacks following
the orchestration [order of execution](../orchestration/index.md). 

The `run` command allows you to filter for a specific set of stacks such as:
- changed stacks
- stacks with or without specific tags
- stacks in a specific directory

For details on how the change detection and order of execution works in Terramate please see:

- [Change Detection](../change-detection/index.md)
- [Orchestration](../orchestration/index.md)

## Usage

`terramate run [options] CMD`

## Examples

Run a command in all stacks:

```bash
terramate run terraform init
```

Run a command in all stacks inside a specific directory:

```bash
terramate run --chdir stacks/aws -- terraform init
```

Run a command in all stacks that [contain changes](../change-detection/index.md):

```bash
terramate run --changed -- terraform init
```

Run a command in all stacks that contain changes and specific tags:

```bash
terramate run  --changed --tags type:k8s -- kubectl diff
```

Run a command in all stacks that don't contain specific tags, with reversed [order of execution](../orchestration/index.md):

```bash
terramate run  --reverse --no-tags type:k8s -- terraform apply
```

Run a command that has its command name and arguments evaluated from an HCL string
interpolation:

```bash
terramate run --eval -- '${global.my_default_command}' '--stack=${terramate.stack.path.absolute}'
```

When using `--eval` the arguments can reference `terramate`, `global` and `tm_` functions with the exception of filesystem related functions (`tm_file`, `tm_fileset`, etc are exposed).

## Options

- `-B, --git-change-base=STRING` Git base ref for computing changes
- `-c, --changed` Filter by changed infrastructure
- `--tags=TAGS` Filter stacks by tags. Use ":" for logical AND and "," for logical OR. Example: --tags `app:prod` filters stacks containing tag "app" AND "prod". If multiple `--tags` are provided, an OR expression is created. Example: `--tags a --tags b` is the same as `--tags a,b`
- `--no-tags=NO-TAGS,...` Filter stacks that do not have the given tags
- `--disable-check-gen-code` Disable outdated generated code check
- `--disable-check-git-remote` Disable checking if local default branch is updated with remote
- `--continue-on-error` Continue executing in other stacks in case of error
- `--no-recursive` Do not recurse into child stacks
- `--dry-run` Plan the execution but do not execute it
- `--reverse` Reverse the order of execution
- `--eval` Evaluate command line arguments as HCL strings

## Project wide `run` configuration.

The `terramate` block at the project root can be used to customize
the default exported environment variables in the 
[terramate.config.run.env](../configuration/project-config.md#the-terramateconfigrunenv-block).

It's also possible to set a different `PATH` environment variable and
in this case, Terramate will honor it when looking up the program's
absolute path.

For example, let's say you have a `bin` directory at the root of the
Terramate project where you define some scripts that should be ran in
each stack. In this case, you can have declaration below in the root
directory:

```hcl
terramate {
  config {
    run {
      env {
        # prepend the bin/ directory so it has preference.
        PATH = "${terramate.root.path.fs.absolute}/bin:${env.PATH}"
      }
    }
  }
}
```

Then if you have the script `bin/create-stack.sh`, you can do:

```bash
$ terramate run create-stack.sh
```

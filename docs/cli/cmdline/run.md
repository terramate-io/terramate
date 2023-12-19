---
title: terramate run - Command
description: With the terramate run command you can execute any command in a single or a list of stacks.
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
terramate run  --changed --tags k8s:prd -- kubectl diff
```

Run a command in all stacks that don't contain specific tags, with reversed [order of execution](../orchestration/index.md):

```bash
terramate run  --reverse --no-tags k8s -- terraform apply
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
[terramate.config.run.env](../projects/configuration.md#the-terramateconfigrunenv-block).

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

## Terramate Cloud functionality

The run command offers extended functionality for Terramate Cloud users. For these commands to work `terramate` must be successfully authenticated with Terramate Cloud. This can be done locally with the [cloud login](./cloud-login.md) command or by creating a trust relationship with Github. In the latter case, you must export the `GITHUB_TOKEN` in the Github action:

```
permissions:
  id-token: write # (necessary for GITHUB_TOKEN to work)
env:
  GITHUB_TOKEN: ${{ github.token }}
```

### Sending deployment data to Terramate Cloud

The `--cloud-sync-deployment` flag will send information about the deployment to Terramate Cloud.

```
jobs:
  deploy:
    name: Deploy
    ...
      - name: Apply changes
        id: apply
        run: terramate run --changed --cloud-sync-deployment -- terraform apply -input=false -auto-approve
```

### Detecting Drift

The `run` command supports `--cloud-sync-drift-status` which will set the Terramate Cloud status of any stack to drifted _if the exit code of the command that is run is `2`_ (which for `terraform plan -detailed-exitcode` signals that the plan succeeded and there was a diff). Terramate is also able to send the drifted plan with the `--cloud-sync-terraform-plan-file` option. A typical Github action for drift detection would look something like:

```
name: Check drift on all stacks once a day

on:
  schedule:
    - cron: '0 2 * * *'

jobs:
  drift-detect:
    name: Check Drift

    permissions:
      id-token: write # necessary for GITHUB_TOKEN to work

    env:
      GITHUB_TOKEN: ${{ github.token }}

    steps:
      ...
      initial setup steps
      ...
      - name: Run drift detection
        id: drift
        run: |
          terramate run --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan -- terraform plan -out drift.tfplan -detailed-exitcode
```

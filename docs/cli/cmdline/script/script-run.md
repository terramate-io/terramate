---
title: terramate script run - Command
description: Execute a Terramate Script in all stacks or in a filtered subset of stacks by using the `terramate script run` command.
---

# Script Run

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["scripts"]`

The `terramate script run` command will run a Terramate script over a set of stacks. `CMD` needs to exactly match the label defined in the `script` block. For example:

```
script "mycommand" {
  ...
}
```

The above script could therefore be run with `script run mycommand`.

The script will run on all stacks under the current working directory where:

- the script is available (scripts follow the same inheritance rules as globals)
- any filters match. `script run` currently supports `--changed` and `--tags` filters.

It's also possible to define commands that consist of multiple keywords to create a multi-level command structure, i.e. `command subcommand ...`:

```
script "command" "subcommand" {
  ...
}
```

The above can be run with `script run command subcommand`.

## Usage

`terramate script run [options] CMD...`

## Examples

Run a script called "deploy" on all stacks where it is available:

```bash
terramate script run deploy
```

Run a script called "deploy" on all changed stacks where it is available:

```bash
terramate script run --changed deploy
```

Run a script called "deploy" on all changed stacks and continue on error:

```bash
terramate script run --changed --continue-on-error deploy
```

Do a dry run of running the deploy script:

```bash
terramate script run --dry-run deploy
```

Run a script called "deploy" in a specific stack without recursing into subdirectories:

```bash
terramate -C path/to/stack script run --no-recursive deploy
```

Run a script in all stacks with an specific Terramate Cloud status:

```bash
terramate script run --cloud-status=unhealthy deploy
```

Run a script called "destroy" on all stacks in the reverse order:

```bash
terramate script run --reverse destroy
```

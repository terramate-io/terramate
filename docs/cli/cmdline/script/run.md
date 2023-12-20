---
title: terramate script run - Command
description: With the terramate

prev:
  text: 'Script List'
  link: '/cli/cmdline/script/list'

next:
  text: 'Script Tree'
  link: '/cli/cmdline/script/tree'
---

# Script Run

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["scripts"]`

The `script run LABEL...` command will run a Terramate script over a set of stacks. The `LABEL` (one or more) needs to exactly match the labels defined in the `script` block:

```
script "label1" "label2" {
  ...
}
```

The above script could therefore be called with `script run label1 label2`.

The script will run on all stacks under the current working directory where:

- the script is available (scripts follow the same inheritance rules as globals)
- any filters match. `script run` currently supports `--changed` and `--tags` filters.

## Usage

`terramate experimental script run [options] SCRIPT-LABEL...`

## Examples

Run a script called "deploy" on all stacks where it is available:

```bash
terramate experimental script run deploy
```

Run a script called "deploy" on all changed stacks where it is available:

```bash
terramate experimental script run --changed deploy
```

Do a dry run of running the deploy script:

```bash
terramate experimental script run --dry-run deploy
```

Run a script called "deploy" in a specific stack without recursing into subdirectories:

```bash
terramate -C path/to/stack experimental script run --no-recursive deploy
```

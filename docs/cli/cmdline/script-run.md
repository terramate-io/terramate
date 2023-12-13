---
title: terramate script run - Command
description: With the terramate

prev:
  text: 'Script List'
  link: '/cli/cmdline/script-list'

next:
  text: 'Script Tree'
  link: '/cli/cmdline/script-tree'
---

# Script Run

**Note:** This is an experimental command that is likely subject to change in the future.

The `script run SCRIPT-NAME` command will run a Terramate script over a set of stacks. The stacks it will run on will be all stacks where the script is available underneath the current working directory unless additional filters are applied.

## Usage

`terramate experimental script run [options] NAME`

## Examples

Run a script called "deploy" on all stacks where it is availablei:

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
terramate -C path/to/stack experimental script --no-recursive run deploy
```

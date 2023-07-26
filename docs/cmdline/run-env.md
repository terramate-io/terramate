---
title: terramate run-env - Command
description: With the terramate run-env command see all environment variables configured for stacks.

prev:
  text: 'Partial Eval'
  link: '/cmdline/partial-eval'

next:
  text: 'Run Graph'
  link: '/cmdline/run-graph'
---

# Run Env

**Note:** This is an experimental command that is likely subject to change in the future.

The `run-env` command prints all values configured in the `terramate.config.run.env` blocks for all stacks in the current
directory recursively.

## Usage

`terramate experimental run-env [options]`

## Examples

Print all values environment variables configured for stacks and child stacks in the current directory:

```bash
terramate experimental run-env
```

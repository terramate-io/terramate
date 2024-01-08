---
title: terramate runtime-env - Command
description: With the terramate run-env command see all environment variables configured for stacks.
---

# Run Env


The `run-env` command prints all values configured in the `terramate.config.run.env` blocks for all stacks in the current
directory recursively.

## Usage

`terramate debug show runtime-env [options]`

## Examples

Print all values environment variables configured for stacks and child stacks in the current directory:

```bash
terramate debug show runtime-env
```

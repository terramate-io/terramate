---
title: terramate debug show runtime-env - Command
description: Debug stack runtime ENV by dumping the available variables and the corresponding values on the stack level by using the 'terramate debug show runtime-env' command.
---

# Run Env

The `terramate debug show runtime-env` command prints all values configured in the `terramate.config.run.env` blocks for all stacks in the current
directory recursively.

## Usage

`terramate debug show runtime-env [options]`

## Examples

Print all values environment variables configured for stacks and child stacks in the current directory:

```bash
terramate debug show runtime-env
```

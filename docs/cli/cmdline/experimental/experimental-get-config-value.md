---
title: terramate experimental get-config-value - Command
description: Debug configuration values by using the `terramate experimental get-config-value` command.
---

# Get Config Value

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental get-config-value` command prints the value of a specific configuration parameter for a stack.

## Usage

`terramate experimental get-config-value <value>`

## Examples

Return the stack name for a specific stack:

```bash
terramate experimental get-config-value 'terramate.stack.name'
```

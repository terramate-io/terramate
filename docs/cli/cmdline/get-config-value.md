---
title: terramate get-config-value - Command
description: With the terramate get-config-value command you can print the value of a specific configuration parameter.
---

# Get Config Value

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `get-config-value` command prints the value of a specific configuration parameter for a stack.

## Usage

`terramate experimental get-config-value`

## Examples

Return the stack name for a specific stack:

```bash
terramate get-config-value 'terramate.stack.name'
```

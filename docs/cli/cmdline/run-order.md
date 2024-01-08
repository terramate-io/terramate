---
title: terramate run-order - Command
description: With the terramate run-order command you can view the order of execution of all stacks.
---

# Run Order

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `run-order` command returns a list that describes the [order of execution](../orchestration/index.md)
of all stacks in the current directory. 

## Usage

`terramate experimental run-order`

## Examples

Show the order of execution of all stacks on the current directory:

```bash
terramate experimental run-order
```

Show the order of execution in a specific directory other than the current:

```bash
terramate experimental run-order --chdir stacks/example
```

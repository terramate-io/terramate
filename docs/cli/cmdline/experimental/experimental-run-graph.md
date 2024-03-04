---
title: terramate experimental run-graph - Command
description: Debug the order of execution by creating a dot graph using the `terramate experimental run-graph` command.
---

# Run Graph

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental run-graph` command prints a graph describing the [order of execution](../../orchestration/index.md) of your stacks.

## Usage

`terramate experimental run-graph`

## Examples

Print the graph for all stacks in the current directory recursively:

```bash
terramate experimental run-graph
```

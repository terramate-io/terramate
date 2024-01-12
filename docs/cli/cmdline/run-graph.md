---
title: terramate run-graph - Command
description: With the terramate run-graph command you can print a graph describing the order of execution of your stacks.
---

# Run Graph

::: warning
This is an experimental command and is likely subject to change in the future.
:::


The `run-graph` command prints a graph describing the [order of execution](../orchestration/index.md) of your stacks.

## Usage

`terramate debug render run-graph`

## Examples

Print the graph for all stacks in the current directory recursively:

```bash
terramate experimental run-graph
```

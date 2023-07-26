---
title: terramate run-graph - Command
description: With the terramate run-graph command you can print a graph describing the order of execution of your stacks.

prev:
  text: 'Run Env'
  link: '/cmdline/run-env'

next:
  text: 'Run Order'
  link: '/cmdline/run-order'
---

# Run Graph

**Note:** This is an experimental command that is likely subject to change in the future.

The `run-graph` command prints a graph describing the [order of execution](../orchestration/index.md) of your stacks.

## Usage

`terramate experimental run-graph`

## Examples

Print the graph for all stacks in the current directory recursively: 

```bash
terramate experimental run-graph
```

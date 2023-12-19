---
title: terramate globals - Command
description: With the terramate globals command you print all globals used in stacks recursively.
---

# Globals

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `globals` command outputs all globals computed for a stack and all child stacks recursively.

## Usage

`terramate experimental globals [options]`

## Examples

Print globals for the stack in the current directory:

```bash
terramate experimental globals
```

Change the working directory: 

```bash
terramate experimental globals --chdir stacks/example
```

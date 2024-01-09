---
title: terramate globals - Command
description: With the terramate globals command you print all globals used in stacks recursively.
---

# Globals

The `globals` command outputs all globals computed for a stack and all child stacks recursively.

## Usage

`terramate debug show globals [options]`

## Examples

Print globals for the stack in the current directory:

```bash
terramate debug show globals
```

Change the working directory:

```bash
terramate debug show globals --chdir stacks/example
```

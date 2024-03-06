---
title: terramate debug show globals - Command
description: Debug Globals in stacks by dumping the available variables and the corresponding values on the stack level by using the 'terramate debug show globals' command.
---

# Globals

The `terramate debug show globals` command outputs all globals computed for a stack and all child stacks recursively.

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

---
title: terramate debug show metadata - Command
description: Debug stack metadata by dumping the available variables and the corresponding values on the stack level by using the 'terramate debug show metadata' command.
---

# Metadata

The `terramate debug show metadata` command prints information stacks and their metadata in the current directory recursively.

## Usage

`terramate debug show metadata`

## Examples

List all stacks and their metadata in the current directory recursively:

```bash
terramate debug show metadata
```

Explicitly change the working directory:

```bash
terramate debug show metadata --chdir path/to/directory
```

---
title: terramate metadata - Command
description: With the terramate metadata command you can see a list of stacks and their metadata.
---

# Metadata

The `metadata` command prints information stacks and their metadata in the current directory recursively.

## Usage

`terramate debug show metadata [options]`

## Examples

List all stacks and their metadata in the current directory recursively:

```bash
terramate debug show metadata
```

Explicitly change the working directory:

```bash
terramate debug show metadata --chdir path/to/directory
```

---
title: terramate metadata - Command
description: With the terramate metadata command you can see a list of stacks and their metadata.

prev:
  text: 'List'
  link: '/cmdline/list'

next:
  text: 'Partial Eval'
  link: '/cmdline/partial-eval'
---

# Metadata

**Note:** This is an experimental command that is likely subject to change in the future.

The `metadata` command prints information stacks and their metadata in the current directory recursively. 

## Usage

`terramate experimental metadata`

## Examples

List all stacks and their metadata in the current directory recursively:

```bash
terramate experimental metadata
```

Explicitly change the working directory:

```bash
terramate experimental metadata --chdir path/to/directory
```

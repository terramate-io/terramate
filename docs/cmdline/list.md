---
title: terramate list - Command
description: With the terramate list command you can list all stacks in the current directory recursively.

prev:
  text: 'Install Completions'
  link: '/cmdline/install-completions'

next:
  text: 'Metadata'
  link: '/cmdline/metadata'
---

# List

The `list` command lists all Terramate stacks in the current directory recursively.

## Usage

`terramate list`

## Examples

List all stacks in the current directory recursively:

```bash
terramate list
```

Explicitly change the working directory:

```bash
terramate list --chdir path/to/directory
```

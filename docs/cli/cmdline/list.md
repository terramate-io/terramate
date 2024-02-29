---
title: terramate list - Command
description: With the terramate list command you can list all stacks in the current directory recursively.
---

# List

The `list` command lists all Terramate stacks in the current directory recursively. These can be additionally filtered based on Terramate Cloud status with the `--cloud-status=<status>` filter (valid statuses are documented on the [trigger page](./trigger.md))

## Usage

`terramate list`

## Examples

### List all stacks in the current directory recursively:

```bash
terramate list
```

### List all stacks in the current directory sorted by order of execution:

```bash
terramate list --run-order
```

### Explicitly change the working directory:

```bash
terramate list --chdir path/to/directory
```

### List all stacks below the current directory that have a "drifted" status on Terramate Cloud

```bash
terramate list --cloud-status=drifted
```

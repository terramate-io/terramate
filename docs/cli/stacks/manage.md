---
title: Manage Stacks
description: This page gives an overview of all available commands in Terramate CLI that help you maintain and understand your stacks.
---

# Manage stacks

This page provides an overview of all available commands in Terramate CLI that help you maintain and understand your stacks.

## Ensure all stacks have IDs

Ensures that all stacks in the project have IDs. If Terramate detects any stacks with missing IDs, UUIDs will be created
and configured automatically.

```sh
terramate create --ensure-stack-ids
```

## List stacks

The list command lists all Terramate stacks in the current directory recursively.
These can be additionally filtered based on Terramate Cloud status with the `--cloud-status=<status>`
filter (valid statuses are documented on the [trigger page](../cmdline/experimental/experimental-trigger.md)).

### Examples

See the following example to understand how filters can be used with the `list` command.

#### List all stacks

```sh
terramate list
```

#### List all stacks in a given path

```sh
terramate list -C subdir/
```

#### List all stacks filtered by tags

```sh
terramate list --tags terraform,kubernetes
```

#### List all stacks that contain changes

```sh
terramate list --changed
```

#### Combining multiple filters

```sh
terramate list -C subdir/ --tags terraform,kubernetes --changed
```

## Manually mark stacks as changed

The [`trigger`](../cmdline/experimental/experimental-trigger.md) command forcibly marks a stack as "changed" even if it doesn't contain any code
changes according to the [change detection](../change-detection/index.md).

```sh
terramate experimental trigger <stack>
```

For details, please see the [`trigger`](../cmdline/experimental/experimental-trigger.md) documentation.

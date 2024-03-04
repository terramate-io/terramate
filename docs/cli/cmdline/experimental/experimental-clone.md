---
title: terramate experimental clone - Command
description: Clone stacks and child stacks to promote them through environments or easily create new resources duplicating existing resource configurations by using the `terramate experimental clone` command.
---

# Clone

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental clone` command clones stacks and nested stacks from a source to a target directory. Terramate will recursively copy the stack
files and directories, and automatically update the `stack.id` with generated UUIDs for the cloned stacks.

The source directory can be a stack itself, or it can contain stacks in sub-directories.

The flag `--skip-child-stacks` can be set to change this behavior, so stacks in sub-directories will be ignored;
in this case, the source directory itself must be a stack.

## Usage

`terramate experimental clone [options] <source> <target>`

## Examples

Clone a stack `alice` to target `bob`:

```bash
terramate experimental clone stacks/alice stacks/bob
```

Clone all stacks within directory `stacks` to `cloned-stacks`:

```bash
terramate experimental clone stacks cloned-stacks
```

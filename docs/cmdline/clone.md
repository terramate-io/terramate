---
title: terramate clone - Command
description: With the terramate command you can easily clone stacks.

prev:
  text: 'Command Line Interface (CLI)'
  link: '/cmdline/'

next:
  text: 'Cloud Login'
  link: '/cmdline/cloud-login'
---

# Clone

**Note:** This is an experimental command and is likely subject to change in the future.

The `clone` command clones stacks from a source to a target directory. Terramate will recursively copy the stack
files and directories, and automatically update the `stack.id` with generated UUIDs for the cloned stacks.

The source directory can be a stack itself, or it can contain stacks in sub-directories.

The flag `--skip-child-stacks` can be set to change this behaviour, so stacks in sub-directories will be ignored;
in this case, the source directory itself must be a stack.

## Usage

`terramate experimental clone SOURCE TARGET`

## Examples

Clone a stack `alice` to target `bob`:

```bash
terramate experimental clone stacks/alice stacks/bob
```

Clone all stacks within directory `stacks` to `cloned-stacks`:

```bash
terramate experimental clone stacks cloned-stacks
```
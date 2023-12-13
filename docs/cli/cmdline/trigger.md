---
title: terramate trigger - Command
description: With the terramate trigger command you can mark a stack to be considered by the change detection.

prev:
  text: 'Script Tree'
  link: '/cli/cmdline/script-tree'

next:
  text: 'Vendor Download'
  link: '/cli/cmdline/vendor-download'
---

# Trigger

**Note:** This is an experimental command that is likely subject to change in the future.

The `trigger` command creates a trigger that marks a stack for as changed to be
considered by the [change detection](../change-detection/index.md).

Per default, triggers are managed in the `.tmtriggers` directory.

## Usage

`terramate experimental trigger PATH`

## Examples

Create a change trigger for a stack:

```bash
terramate experimental trigger /path/to/stack
```

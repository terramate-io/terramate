---
title: File Watchers Change Detection
description: Learn how to use File Watchers to mark a stack as changed whenever files outside the stack's directory are changed.
---

# File Watchers

Stacks can be configured to watch files for changes that are not part of the stacks directory.
If any of the watched files changes, the stack will be marked as changed in the change detection.

**Example:**

```hcl
stack {
  watch = [
    "/external/file1.txt",
    "/external/file2.txt"
  ]
}
```

For details, please see the [stack configuration](../stacks/configuration.md#stackwatch-listoptional) documentation.

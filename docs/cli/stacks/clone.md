---
title: Clone Stacks
description: Learn how to easily clone stacks and nested stacks using the terramate clone command.
---

# Clone stacks

With Terramate CLI you can easily clone stacks and nested stacks using the [clone](../cmdline/experimental/experimental-clone.md) command.

Imagine you develop a new service in staging that you want to roll out to production. Instead of manually copying and
pasting a bunch of stacks, updating stack properties such as `stack.id`, etc., [clone](../cmdline/experimental/experimental-clone.md) does this
for you automatically.

```sh
terramate experimental clone <source> <target>
```

The `clone` command clones stacks and nested stacks from a source to a target directory. Terramate CLI will recursively
copy the stack files and directories, and automatically update the `stack.id` with generated UUIDs for the cloned stacks.
In addition, the code generation will be triggered to ensure that the generated code for cloned stacks will be up to date.

For details please see the [clone command](../cmdline/experimental/experimental-clone.md) documentation.

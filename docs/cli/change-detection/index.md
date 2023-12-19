---
title: Change Detection
description: Learn about the fundamental concept of Change Detection in Terramate.
---

# Change Detection

When working with multiple stacks, a common challenge is to execute commands in stacks that contain changes only to
preserve a **small blast radius** and **fast execution run times**.

That's why Terramate CLI comes with a change detection feature that can detect stacks containing changes in a commit, branch, or Pull Request.

## Introduction 

The change detection is enabled by providing the `--changed` option to commands such as [`run`](../cmdline/run.md) or
[`list`](../cmdline/list.md) and can be configured to use a specific branch as a reference.

E.g., to list all stacks that contain changes:

```sh
terramate list --changed
```

## Integrations

Detecting changed stacks that contain changes only is based on a [Git integration](./integrations/git.md).

Several other integrations exist to cover specific use cases. For example, the [Terraform integration](./integrations/terraform.md)
allows to mark stacks as changed even if they reference local Terraform modules that have changed but are located outside of a stack directory.

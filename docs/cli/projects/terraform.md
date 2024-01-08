---
title: Add Terramate to existing Terraform Project
description: Learn how to add Terramate to an existing Terraform project with a single command.
---

# Add Terramate to existing Terraform Project

You can easily initialize Terramate in an existing Terraform project.

```bash
terramate create --all-terraform
```

The [`create --all-terraform`](../cmdline/create.md) command scans any Git repository for directories containing Terraform
`backend` or `provider` configuration, which commonly indicates that it's a Terraform stack. A Terramate stack configuration
[`stack.tm.hcl`](../stacks/configuration.md) will be added to each detected Terraform stack so that Terramate can recognize
it as a Terramate stack.

::: info
Whenever you add Terramate to an existing Terraform project, Terramate will **never* modify your existing Terraform configuration.
:::

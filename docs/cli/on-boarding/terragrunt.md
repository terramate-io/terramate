---
title: "On-boarding: Terragrunt On-boarding"
description: Import your existing Terragrunt Setup to Terramate
outline: [2, 3]
---

::: warning
This is an experimental command and is likely subject to change in the future.
It needs to be enabled in the experiments config by adding `terragrunt` to `terramate.config.experiments` list.
:::

## Import Existing Terragrunt Stacks

To create Terramate Stacks from existing Terragrunt Modules run the following command.

```bash
terramate create --all-terragrunt
```

This command will detect existing Terragrunt Modules, create a stack configuration in them and will set up the order of execution in `before` and `after` attributes for detected Terragrunt dependencies.

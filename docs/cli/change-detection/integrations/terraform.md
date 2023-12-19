---
title: Terraform Change Detection Integration
description: Learn how Terramate can help to detect Terraform stacks that reference changed Terraform modules.
---

# Terraform Change Detection Integration

## Module change detection

A Terraform stack can be composed of multiple local modules and if that's the
case then any changes on a module that a stack references will mark the stack as changed.
The rationale is that if any module referenced by a stack changes then the stack itself changed and needs to be re-deployed.

For more details see the example below:

![Module Change Detection](../../assets/module-change-detection.gif)

In order to do that, Terramate will parse all `.tf` files inside the stack and
check if the local modules it depends on have changed.

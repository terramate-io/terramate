---
title: Orchestration
description: Learn how to orchestrate the execution of commands or sequences of commands in stacks using the orchestration in Terramate.
---

# Orchestration

One of the most essential features of Terramate CLI is its ability to orchestrate the execution of commands in stacks, which allows to run commands such as `terraform apply` or `kubectl apply` across different stacks without having to navigate to each project stack individually.

Even in the case of environments where stacks are interdependent, Terramate’s orchestration ensures that operations are performed in the correct order, maintaining dependencies and preventing conflicts.

The orchestration engine provides various strategies for selecting stacks and configuring the execution order, which are explained in the following pages of this section.

## Default order of execution

In Terramate Projects, we can arrange stacks in a filesystem hierarchy. Parent stacks are always executed before their
child stacks in this arrangement. Thus, if stack A includes stack B, stack A will always be executed first.

This provides the ability to rearrange stacks, which can improve the mirroring of your cloud infrastructure and control
the sequence of execution, all without changing any code.

This ordering will fit well with the natural project organization and eliminate
the need for hard-coded dependencies between stacks.

Per default, commands such as [`run`](../cmdline/run.md) or [`list`](../cmdline/list.md) follow the default order of execution
so that parent stacks will run before child stacks.

The default order of execution can be altered in a stack's configuration. For details, please see
[configuring the order of execution](../stacks/configuration.md#configuring-the-order-of-execution).

::: tip
You can use the [run-order](../cmdline/run-order.md)
command to understand the order of execution of your stacks.
:::

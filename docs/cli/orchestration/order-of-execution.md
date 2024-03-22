---
title: Order of Execution
description: Learn about the order of execution when orchestration stacks with Terramate.
---

# Order of Execution

## Implicit Order of Execution

Implicit order will be detected by Terramate depending on integrations and hierarchy of stacks.

### Parent and Child Stacks

Currently Terramate orders stacks implicitly when they are nested.
nested stacks are stacks that are in a subdirectory of stack.

Any level of nesting is supported.

Parent stacks will be ordered `before` child stacks.

## Explicit Order of Execution

In addition to automatical detected order via implicit rules, stacks can be configured to define an explicit order of execution.

Stacks can be configured to be executed `before` a set of stacks and/or `after` a set of stacks.

This order can be defined at stack creation time or changed at any point in time.

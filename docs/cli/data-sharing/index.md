---
title: Sharing Data - Overview
description: Terramate enables you to define data once and distribute it throughout your project. This can be accomplished using globals and metadata.

prev:
  text: 'Change Detection'
  link: '/cli/change-detection/'

next:
  text: 'Code Generation'
  link: '/cli/code-generation/'
---

# Sharing Data in Terramate

Maintaining [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
(Don't Repeat Yourself) principles in your code is crucial to keep your
project neat and manageable. Terramate facilitates this practice by enabling you to define
data once and distribute it throughout your project. This can be accomplished using two
main constructs: Globals and Metadata.

[Globals](./globals.md) are user-defined entities, similar to locals in Terraform, whereas 
[Metadata](./metadata.md) is information supplied by Terramate itself. These are integrated with 
Terraform through a code generation process. To delve deeper into code generation process, read [here](../code-generation/index.md).

# Lazy Evaluation in Terramate

Given that globals can reference other globals and Terramate metadata, it is
important to be clear about how/when evaluation happens.

Globals are lazily evaluated, which means globals block can reference variables
that will exist at a later point in time. For example, a non-stack directory
can have globals referencing stack metadata.

Below is a high-level overview of how globals evaluation is implemented for a given
stack:

- The globals blocks are loaded from all Terramate files in the stack directory.
- The multiple globals blocks are merged into a single globals definition.
- Recursively, the parent directories have their globals blocks loaded and merged into
the same single globals definition but giving preference for the global defined lower in
the filesystem tree.
- Then resulting single globals block have its attributes evaluated.
  - The `terramate.*` namespace is set with the stack [Metadata](./metadata.md) values.
  - Globals that depend on other globals are postponed until it's dependencies are evaluated.

This means that globals can reference globals on other configurations independent of how specific or 
general the configuration is since it is all merged together into a single globals set before evaluation.

---
title: Variables
description: Learn how to define and use variables such as metadata, globals or temporary and context-based lets variables.
---

# Variables

## Introduction

Terramate supports different Variables and Metadata to help manage user- and Terramate-defined data.

## Variable Namespaces

Terramate supports multiple variable namespaces. They can be available at build-time (when running
[code generation](../index.md)) or run-time (when orchestrating stacks and
[running commands](../../orchestration/run-commands-in-stacks.md)).

- The `terramate` namespace represents [Terramate Metadata](./metadata.md) such as stack context information or repository context information.
- The `global` namespace represents [Global Variables](./globals.md) defined with the `globals` block.
- The `let` namespace represents context-based [Lets Variables](./lets.md) that can be used to compute local blocks available in the
current code generation block only to not pollute the `global` namespace with temporary or intermediate variables.
- The `env` namespace is only available at run-time and represents the commands
[environment variables](../../orchestration/runtime-configuration.md) exported by a shell.

The following sections explain the different types of Variables in Terramate.

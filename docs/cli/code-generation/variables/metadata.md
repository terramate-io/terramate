---
title: Metadata
description: Learn how stacks help you to split up your infrastructure code and configuration such as Terraform into isolated units.
---

# Terramate Metadata Variables

Terramate supports access to metadata of stacks, projects and the repository in the `terramate` namespace.

## Project Metadata

Project metadata is the same independent of a stack.

- The `terramate` object provides access to project metadata.
  - `version` (string) The Terramate version.
  - `stacks.list` (list of strings) List of all stacks inside the project. Each stack is represented by its absolute path
  relative to the project root. The list will be ordered lexicographically.
  - `root.path.fs.absolute` (string) The absolute path of the project root directory. Will be the same for all stacks.
  - `root.path.fs.basename` (string) The base name of the project root directory. Will be the same for all stacks.

## Stack Metadata

- The `terramate.stack` object grants access to stack metadata and is only available in the stack context.
The following keys are available in the `terramate.stack` object and can be accessed with `terramate.stack.<key>`, e.g. `terramate.stack.id`.
  - `id` (string) The user-defined `id` of the stack.
  - `name` (string) The user-defined `name` of the stack.
  - `description` (string) The user-defined `description` of the stack.
  - `tags` (list of string) The user-defined `tags` of the stack.
  - `path` (object) An object defining the path of a stack within the repository in different ways
    - `absolute` (string) The absolute path of the stack within the repository.
    - `basename` (string) The base name of the stack path.
    - `relative` (string) The relative path of the stack from the repository root.
    - `to_root` (string) The relative path from the stack to the repository root (upwards).

## Repository Metadata

- The `terramate.stacks` object grants access to a list of all stacks
The following keys are available in the `terramate.stacks.` object and can be accessed with `terramate.stacks.<key>`, e.g. `terramate.stack.list:`
  - `list` (list of string) A list of all absolute stack paths in the current repository.

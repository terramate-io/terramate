---
title: Stacks | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Getting Started'
  link: '/getting-started/'

next:
  text: 'Orchestration'
  link: '/orchestration/'
---

# Stack Configuration

When working with Infrastructure as Code it's considered to be a best practice
to split up and organize your IaC into several smaller and isolated stacks.

Typically, each stack comes with its own Terraform state which allows us
to plan and apply each stack on its own.

A Terramate stack is:

* A directory inside your project.
* Has at least one or more Terramate configuration files.
* One of the configuration files has a `stack {}` block on it.

What separates a stack from any other directory is the `stack{}` block.
It doesn't require any attributes by default, but it can be used
to describe stacks and orchestrate their execution.

Stack configurations related to orchestration can be found [here](../orchestration/index.md).

Besides orchestration the `stack` block also define attributes that are
used to describe the `stack`.

Only [Terramate Functions](../functions/index.md) are available when defining
the `stack` block.

## stack.id (string)(optional)

The stack ID **must** be a string composed of alphanumeric chars + `-` + `_`.
The ID can't be bigger than 64 bytes and **must** be unique on the
whole project.

There is no default value determined for the stack ID.

Eg:

```hcl
stack {
  id = "some_id_that_must_be_unique"
}
```

## stack.name (string)(optional)

The stack name can be any string and it defaults to the stack directory
base name.

Eg:

```hcl
stack {
  name = "My Awesome Stack Name"
}
```

## stack.description (string)(optional)

The stack description can be any string and it defaults to an empty string.

Eg:

```hcl
stack {
  description = "My Awesome Stack Description"
}
```

## stack.tags (set(string))(optional)

The tags must be a unique set of strings, where each tag must adhere to the following rules:

- It must start with a lowercase ASCII alphabetic character (`[a-z]`).
- It must end with a lowercase ASCII alphanumeric character (`[0-9a-z]`).
- It must have only lowercase ASCII alphanumeric, `_` and `-` characters (`[0-9a-z_-]`).

## stack.watch (list)(optional)

The list of files that must be watched for changes in the
[change detection](../change-detection/index.md).

## stack.after (set(string))(optional)

The `after` defines the list of stacks which this stack must run after.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md).
See [orchestration docs](../orchestration/index.md#stacks-ordering) for details.

## stack.before (set(string))(optional)

The `before` defines the list of stacks which this stack must run before.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md).
See [orchestration docs](../orchestration/index.md#stacks-ordering) for details.

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

When working with Infrastructure as Code (IaC), adopting a modular approach is highly recommended. This approach breaks the entire IaC into smaller, isolated **stacks**, enabling code and infrastructure component reuse. 

Additionally, it enhances infrastructure management across multiple stacks and facilitates testing and deploying changes in a controlled manner, minimizing unintended consequences.

The Terramate stack is a powerful feature that allows you to manage complex deployments.


## What is a Stack?

A stack is a collection of related resources managed as a single unit. When defining a stack, you specify all included resources, such as networks, virtual machines, and storage. 

Terramate Stack simplifies resource creation and management, allowing you to focus on building and deploying the application.


## A Terramate stack is:

- A directory inside your project
- Contains one or more Terramate configuration files
- Includes a configuration file with a stack{} block

The `stack{}` block distinguishes a stack from other directories in Terramate. By default, it doesn't require any attributes but can be used to describe the stack and orchestrate its execution.

Stack configurations related to orchestration can be found [here](../orchestration/index.md). 


## Why use Stacks?

Stacks offer several benefits:

1. Manage multiple resources as one unit, allowing you to build and deploy the entire infrastructure efficiently and quickly with a single command.

2. Easily manage dependencies between resources. For example, you can specify a virtual machine that depends on a virtual network, and Terramate CLI will be used to manage the resource's creation or deletion.

3. Simplify infrastructure management as code. Defining the entire infrastructure using Terraform configuration files makes version control and change management more accessible.


## Creating a Stack:

To create a stack using Terramate, follow these steps:

1. Define the included resources in a Terraform configuration file. This file should contain all resources you want to manage as part of the stack.

2. Use the following command to create the resources:

```hcl
terramate create
```
Running this command creates creates the `stack.tm.hcl` file containing the stack block.


# Properties

Each stack has a set of properties that can be accessed and used while building your IaC. 

Some of these properties include:

## stack.id (string)(optional)

The stack ID **must** be a string composed of alphanumeric chars + `-` + `_`.
The ID can't be bigger than 64 bytes and **must** be unique on the
whole project.

There is no default value determined for the stack ID.

```hcl
stack {
  id = "some_id_that_must_be_unique"
}
```

## stack.name (string)(optional)

The stack name can be any string and defaults to the stack directory base name.

```hcl
stack {
  name = "My Awesome Stack Name"
}
```

## stack.description (string)(optional)

The stack description can be any string and defaults to an empty string.

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

```hcl
stack {
  tags = [
    "aws",
    "vpc",
    "bastion",
  ]
}
```

## stack.watch (list)(optional)

The list of files that must be watched for changes in the
[change detection](../change-detection/index.md).

```hcl
stack {
  watch = [
    "/policies/mypolicy.json"
  ]
}
```

The configuration above will mark the stack as changed whenever
the file `/policies/mypolicy.json` changes.

## stack.after (set(string))(optional)

The `after` defines the list of stacks which this stack must run after.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md).

```hcl
stack {
  after = [
    "tag:prod:networking",
    "/prod/apps/auth"
  ]
}
```

The stack above will run after all stacks tagged with `prod` **and** `networking` and after `/prod/apps/auth` stack.

See [orchestration docs](../orchestration/index.md#stacks-ordering) for details.

## stack.before (set(string))(optional)

Defines the list of stacks that this stack must run `before`. It accepts project absolute paths (like `/other/stack`), paths relative to the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md). See  [orchestration docs](../orchestration/index.md#stacks-ordering) for details.

## stack.wants (set(string))(optional)

This attribute defines the list of stacks that must be selected 
whenever this stack is selected to be executed.

```hcl
stack {
  wants = [
    "/other/stack"
  ]
}
```

When the stack defined above is selected to be executed, the list
of stacks defined in its `wants` set are also selected.

Suppose you need to run just a subset of the project's stacks, 
you can do so by `cd` (change directory) into a child directory.
In that case, when executing `terramate run` only the stacks visible
from that directory are going to be executed, but if any of the
selected stacks have a `wants` clause for selecting additional stacks,
not present in the subset, then they will also be included in the
final execution set.

## stack.wanted_by (set(string))(optional)

This attribute defines the list of stacks that are wanted by
this stack, which means the stacks in the list will select this
stack whenever they are selected for execution.

```
stack {
  wanted_by = [
    "/other/stack-1",
    "/other/stack-2",
  ]
}
```

When using the configuration above, whenever `/other/stack-1` or
`/other/stack-2` is selected to be executed, then Terramate will
also select the current stack.
This option works in the same way as if both `/other/stack-1` and 
`/other/stack-2` had a `stack.wants` attribute targeting this stack.
